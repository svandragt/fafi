package bookmark

import (
	"database/sql"
	"errors"
	"fafi2/sander"
	"github.com/mattn/go-sqlite3"
	"log"
	"strconv"
	"strings"
	"time"
)

var (
	ErrDuplicate    = errors.New("record already exists")
	ErrNotExists    = errors.New("row not exists")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
)

// schemaVersion is the latest schema this binary writes. Bump and add a
// migration step in MigrateSchema when changing schema shape.
const schemaVersion = 3

type Database struct {
	db      *sql.DB
	version int
}

func NewDatabase(db *sql.DB) *Database {
	return &Database{
		db: db,
	}
}

// MigrateSchema brings the database up to schemaVersion.
//
// Strategy:
//   - empty DB (user_version=0): create at the latest version directly.
//   - v1 (the original FTS5 schema + bookmark_meta sibling table): leave
//     in place. v1 → latest migration only runs on explicit FAFI_RESET_INDEX
//     since it forces a full re-index (text + isScraped wiped).
//   - v2 → v3: run dedup sweep automatically (cheap, idempotent).
//   - already at latest: no-op.
func (r *Database) MigrateSchema() error {
	v, err := r.userVersion()
	if err != nil {
		return err
	}

	switch v {
	case 0:
		if err := r.createV2(); err != nil {
			return err
		}
		if err := r.setUserVersion(schemaVersion); err != nil {
			return err
		}
		r.version = schemaVersion
		log.Printf("Database created (v%d)\n", schemaVersion)
	case 1:
		r.version = 1
		// Auto-migrate forward. Safe because migrateToV2 preserves text
		// and isScraped — no reindex required.
		if err := r.migrateToV2(); err != nil {
			return err
		}
		if err := r.migrateV2toV3(); err != nil {
			return err
		}
	case 2:
		r.version = 2
		if err := r.migrateV2toV3(); err != nil {
			return err
		}
	case schemaVersion:
		r.version = schemaVersion
	default:
		return errors.New("unknown schema version")
	}
	log.Printf("Schema version: v%d", r.version)
	return nil
}

// Version returns the schema version currently in use.
func (r *Database) Version() int {
	return r.version
}

func (r *Database) userVersion() (int, error) {
	var v int
	if err := r.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, err
	}
	// user_version=0 + existing bookmarks table means a legacy v1 install
	// that predates versioning.
	if v == 0 {
		var name string
		err := r.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name='bookmarks'",
		).Scan(&name)
		if err == nil {
			return 1, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
	}
	return v, nil
}

func (r *Database) setUserVersion(v int) error {
	// PRAGMA does not accept bound parameters.
	_, err := r.db.Exec("PRAGMA user_version = " + strconv.Itoa(v))
	return err
}

func (r *Database) createV2() error {
	_, err := r.db.Exec(`
	CREATE VIRTUAL TABLE bookmarks USING FTS5(
	    url,
	    title,
	    text,
	    content_type,
	    isScraped UNINDEXED,
	    dateAdded UNINDEXED
	);
	`)
	return err
}

// migrateV2toV3 collapses http/https duplicates into the https variant.
// Where the https row is missing scraped content but the http one has it,
// the content is moved over before the http row is deleted.
//
// Idempotent: runs on every boot at v2; becomes a no-op once swept (and
// bumps user_version=3 so the boot path skips it next time).
func (r *Database) migrateV2toV3() error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Find http URLs whose https counterpart also exists.
	rows, err := tx.Query(`
		SELECT b1.url
		FROM bookmarks b1
		WHERE b1.url LIKE 'http://%'
		  AND EXISTS (
		      SELECT 1 FROM bookmarks b2
		      WHERE b2.url = 'https://' || substr(b1.url, 8)
		  )`)
	if err != nil {
		return err
	}
	var httpURLs []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			_ = rows.Close()
			return err
		}
		httpURLs = append(httpURLs, u)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, httpURL := range httpURLs {
		httpsURL := "https://" + httpURL[len("http://"):]
		// Merge content from http into https if https has no scraped data yet.
		if _, err := tx.Exec(`
			UPDATE bookmarks
			SET text = (SELECT text FROM bookmarks WHERE url = ?),
			    content_type = (SELECT content_type FROM bookmarks WHERE url = ?),
			    isScraped = (SELECT isScraped FROM bookmarks WHERE url = ?)
			WHERE url = ?
			  AND (text IS NULL OR text = '')
			  AND (SELECT text FROM bookmarks WHERE url = ?) IS NOT NULL
			  AND (SELECT text FROM bookmarks WHERE url = ?) <> ''`,
			httpURL, httpURL, httpURL, httpsURL, httpURL, httpURL,
		); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM bookmarks WHERE url = ?", httpURL); err != nil {
			return err
		}
	}

	if _, err := tx.Exec("PRAGMA user_version = 3"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.version = 3
	if len(httpURLs) > 0 {
		log.Printf("Deduplicated %d http/https bookmark pair(s)", len(httpURLs))
	}
	return nil
}

// migrateToV2 rebuilds the FTS table at v2 and drops the sibling meta table.
// Preserves url, title, dateAdded; clears text and isScraped so the indexer
// re-fetches every bookmark (which also runs the new content-type probe).
func (r *Database) migrateToV2() error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
	CREATE VIRTUAL TABLE bookmarks_v2 USING FTS5(
	    url,
	    title,
	    text,
	    content_type,
	    isScraped UNINDEXED,
	    dateAdded UNINDEXED
	);
	`); err != nil {
		return err
	}
	// Preserve scraped text + isScraped so rows that have already been
	// fetched don't need a reindex. content_type is left empty here; the
	// indexer's probe step will populate it next time SelectQueue picks
	// the row up (which only happens for rows that were never scraped,
	// or after FAFI_RESET_INDEX clears isScraped).
	if _, err := tx.Exec(
		`INSERT INTO bookmarks_v2 (url, title, text, content_type, isScraped, dateAdded)
		 SELECT url, title, COALESCE(text, ''), '', isScraped, dateAdded FROM bookmarks`,
	); err != nil {
		return err
	}
	if _, err := tx.Exec("DROP TABLE bookmarks"); err != nil {
		return err
	}
	if _, err := tx.Exec("ALTER TABLE bookmarks_v2 RENAME TO bookmarks"); err != nil {
		return err
	}
	if _, err := tx.Exec("DROP TABLE IF EXISTS bookmark_meta"); err != nil {
		return err
	}
	if _, err := tx.Exec("PRAGMA user_version = 2"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.version = 2
	log.Println("Database migrated to v2")
	return nil
}

func (r *Database) CreateOrGet(bm Bookmark) (*Bookmark, error) {
	bm.URL = NormalizeURL(bm.URL)

	existingBookmark, err := BmDb.GetByUrl(bm.URL)
	if existingBookmark != nil {
		return existingBookmark, err
	}

	// Scheme-only dedup: treat http://X and https://X as the same bookmark,
	// always preferring the https variant.
	if other := otherSchemeURL(bm.URL); other != "" {
		otherBm, _ := BmDb.GetByUrl(other)
		if otherBm != nil {
			if strings.HasPrefix(bm.URL, "http://") {
				// Inserting http when https already exists → return https row.
				return otherBm, nil
			}
			// Inserting https when http exists → drop http row, fall through
			// and insert the https one.
			if err := r.Delete(other); err != nil {
				log.Println("dedup delete error:", err)
			}
		}
	}

	bm.DateAdded = SqlTime(time.Now())

	var query string
	var args []any
	if r.version >= 2 {
		query = `INSERT INTO bookmarks (url, title, text, content_type, dateAdded) VALUES (?, ?, ?, ?, ?);`
		args = []any{bm.URL, bm.Title, bm.Text, bm.ContentType, bm.DateAdded.String()}
	} else {
		query = `INSERT INTO bookmarks (url, title, text, dateAdded) VALUES (?, ?, ?, ?);`
		args = []any{bm.URL, bm.Title, bm.Text, bm.DateAdded.String()}
	}
	_, err = r.db.Exec(query, args...)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
				return nil, ErrDuplicate
			}
		}
		return nil, err
	}

	log.Println("Created:", bm.URL)
	return &bm, nil
}

func (r *Database) CreateMany(bms []Bookmark) {
	for _, bm := range bms {
		_, err := r.CreateOrGet(bm)
		if err != nil {
			log.Fatal("Error creating bookmark:", err)
			return
		}
	}
}

func (r *Database) All(keywords string) ([]Bookmark, error) {
	if r.version >= 2 {
		return r.allV2(keywords)
	}
	return r.allV1(keywords)
}

func (r *Database) allV2(keywords string) ([]Bookmark, error) {
	var rows *sql.Rows
	var err error
	if keywords != "" {
		//goland:noinspection SqlSignature,SqlResolve
		rows, err = r.db.Query(`
			SELECT
				url,
				title,
				snippet(bookmarks, 2, ?, ?, '...', 64) as text,
				content_type,
				isScraped,
				dateAdded
			FROM bookmarks
			WHERE text is not '' AND (
				title MATCH ? OR
				url MATCH ? OR
				text MATCH ?
			)
			ORDER BY rank
			LIMIT ?`,
			"[", "]", keywords, keywords, keywords, 50,
		)
	} else {
		rows, err = r.db.Query(
			`SELECT url, title, text, content_type, isScraped, dateAdded
			 FROM bookmarks
			 WHERE text is not ''
			 ORDER BY dateAdded DESC, title
			 LIMIT 50`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var all []Bookmark
	for rows.Next() {
		var bm Bookmark
		var ct sql.NullString
		if err := rows.Scan(&bm.URL, &bm.Title, &bm.Text, &ct, &bm.IsScraped, &bm.DateAdded); err != nil {
			return nil, err
		}
		if ct.Valid {
			bm.ContentType = ct.String
		}
		all = append(all, bm)
	}
	log.Println(len(all), sander.Pluralize("result", len(all)))
	return all, nil
}

func (r *Database) allV1(keywords string) ([]Bookmark, error) {
	var rows *sql.Rows
	var err error
	if keywords != "" {
		//goland:noinspection SqlSignature,SqlResolve
		rows, err = r.db.Query(`
			SELECT
				b.url,
				b.title,
				snippet(bookmarks, 2, ?, ?, '...', 64) as text,
				b.isScraped,
				b.dateAdded,
				m.content_type
			FROM bookmarks b
			LEFT JOIN bookmark_meta m ON m.url = b.url
			WHERE b.text is not '' AND (
				b.title MATCH ? OR
				b.url MATCH ? OR
				b.text MATCH ?
			)
			ORDER BY rank
			LIMIT ?`,
			"[", "]", keywords, keywords, keywords, 50,
		)
	} else {
		rows, err = r.db.Query(`
			SELECT b.url, b.title, b.text, b.isScraped, b.dateAdded, m.content_type
			FROM bookmarks b
			LEFT JOIN bookmark_meta m ON m.url = b.url
			WHERE b.text is not ''
			ORDER BY b.dateAdded DESC, b.title
			LIMIT 50`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var all []Bookmark
	for rows.Next() {
		var bm Bookmark
		var ct sql.NullString
		if err := rows.Scan(&bm.URL, &bm.Title, &bm.Text, &bm.IsScraped, &bm.DateAdded, &ct); err != nil {
			return nil, err
		}
		if ct.Valid {
			bm.ContentType = ct.String
		}
		all = append(all, bm)
	}
	log.Println(len(all), sander.Pluralize("result", len(all)))
	return all, nil
}

// ResetIndex clears the isScraped flag on every row so the indexer re-processes
// them on the next run. If the database is on an older schema, it is migrated
// to the latest version first (which itself clears isScraped).
func (r *Database) ResetIndex() error {
	if r.version < 2 {
		if err := r.migrateToV2(); err != nil {
			return err
		}
	}
	if r.version < 3 {
		if err := r.migrateV2toV3(); err != nil {
			return err
		}
	}
	_, err := r.db.Exec("UPDATE bookmarks SET isScraped = NULL")
	return err
}

func (r *Database) SelectQueue() ([]Bookmark, error) {
	var rows *sql.Rows
	var err error
	if r.version >= 2 {
		rows, err = r.db.Query(
			`SELECT url, title, text, content_type, isScraped, dateAdded
			 FROM bookmarks WHERE isScraped is not 1 ORDER BY RANDOM()`,
		)
	} else {
		rows, err = r.db.Query(
			`SELECT url, title, text, isScraped, dateAdded FROM bookmarks
			 WHERE isScraped is not 1 ORDER BY RANDOM()`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var all []Bookmark
	for rows.Next() {
		var bm Bookmark
		if r.version >= 2 {
			var ct sql.NullString
			if err := rows.Scan(&bm.URL, &bm.Title, &bm.Text, &ct, &bm.IsScraped, &bm.DateAdded); err != nil {
				if !bm.IsScraped.Valid {
					return all, nil
				}
				return nil, err
			}
			if ct.Valid {
				bm.ContentType = ct.String
			}
		} else {
			if err := rows.Scan(&bm.URL, &bm.Title, &bm.Text, &bm.IsScraped, &bm.DateAdded); err != nil {
				if !bm.IsScraped.Valid {
					return all, nil
				}
				return nil, err
			}
		}
		all = append(all, bm)
	}
	return all, nil
}

func (r *Database) GetByUrl(url string) (*Bookmark, error) {
	var row *sql.Row
	if r.version >= 2 {
		row = r.db.QueryRow(
			`SELECT url, title, text, content_type, isScraped, dateAdded
			 FROM bookmarks WHERE url = ?`, url)
	} else {
		row = r.db.QueryRow(
			`SELECT url, title, text, isScraped, dateAdded
			 FROM bookmarks WHERE url = ?`, url)
	}

	var bm Bookmark
	var err error
	if r.version >= 2 {
		var ct sql.NullString
		err = row.Scan(&bm.URL, &bm.Title, &bm.Text, &ct, &bm.IsScraped, &bm.DateAdded)
		if ct.Valid {
			bm.ContentType = ct.String
		}
	} else {
		err = row.Scan(&bm.URL, &bm.Title, &bm.Text, &bm.IsScraped, &bm.DateAdded)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotExists
		}
		log.Fatal("GetByUrl error:", err)
	}
	return &bm, nil
}

func (r *Database) GetLastDateAddedMicro() int64 {
	row := r.db.QueryRow("SELECT dateAdded FROM bookmarks ORDER BY dateAdded DESC LIMIT 1")

	var bm Bookmark
	if err := row.Scan(&bm.DateAdded); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 100000
		}
		log.Fatal("GetLastDateAdded error:", err)
	}
	st := time.Time(bm.DateAdded)
	return st.UnixMicro()
}

func (r *Database) Update(url string, updated Bookmark) (*Bookmark, error) {
	var res sql.Result
	var err error
	if r.version >= 2 {
		res, err = r.db.Exec(
			"UPDATE bookmarks SET title = ?, text = ?, content_type = ?, isScraped = ? WHERE url = ?",
			updated.Title, updated.Text, updated.ContentType, updated.IsScraped, url,
		)
	} else {
		res, err = r.db.Exec(
			"UPDATE bookmarks SET title = ?, text = ?, isScraped = ? WHERE url = ?",
			updated.Title, updated.Text, updated.IsScraped, url,
		)
		if err == nil && updated.ContentType != "" {
			// v1 stores content_type in the sibling table.
			if _, mErr := r.db.Exec(
				`INSERT INTO bookmark_meta (url, content_type) VALUES (?, ?)
				 ON CONFLICT(url) DO UPDATE SET content_type = excluded.content_type`,
				url, updated.ContentType,
			); mErr != nil {
				log.Println("bookmark_meta upsert error:", mErr)
			}
		}
	}
	if err != nil {
		return nil, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected == 0 {
		return nil, ErrUpdateFailed
	}

	return &updated, nil
}

func (r *Database) Delete(url string) error {
	res, err := r.db.Exec("DELETE FROM bookmarks WHERE url = ?", url)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrDeleteFailed
	}

	if r.version < schemaVersion {
		_, _ = r.db.Exec("DELETE FROM bookmark_meta WHERE url = ?", url)
	}

	return err
}

var BmDb *Database
