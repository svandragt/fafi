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
const schemaVersion = 7

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
		if err := r.createStatusTable(); err != nil {
			return err
		}
		if err := r.createDeletedTable(); err != nil {
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
		if err := r.migrateV3toV4(); err != nil {
			return err
		}
		if err := r.migrateV4toV5(); err != nil {
			return err
		}
		if err := r.migrateV5toV6(); err != nil {
			return err
		}
		if err := r.migrateV6toV7(); err != nil {
			return err
		}
	case 2:
		r.version = 2
		if err := r.migrateV2toV3(); err != nil {
			return err
		}
		if err := r.migrateV3toV4(); err != nil {
			return err
		}
		if err := r.migrateV4toV5(); err != nil {
			return err
		}
		if err := r.migrateV5toV6(); err != nil {
			return err
		}
		if err := r.migrateV6toV7(); err != nil {
			return err
		}
	case 3:
		r.version = 3
		if err := r.migrateV3toV4(); err != nil {
			return err
		}
		if err := r.migrateV4toV5(); err != nil {
			return err
		}
		if err := r.migrateV5toV6(); err != nil {
			return err
		}
		if err := r.migrateV6toV7(); err != nil {
			return err
		}
	case 4:
		r.version = 4
		if err := r.migrateV4toV5(); err != nil {
			return err
		}
		if err := r.migrateV5toV6(); err != nil {
			return err
		}
	case 5:
		r.version = 5
		if err := r.migrateV5toV6(); err != nil {
			return err
		}
		if err := r.migrateV6toV7(); err != nil {
			return err
		}
	case 6:
		r.version = 6
		if err := r.migrateV6toV7(); err != nil {
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

// createDeletedTable creates the tombstone table that records soft-deleted
// bookmarks so re-imports don't re-add them.
func (r *Database) createDeletedTable() error {
	_, err := r.db.Exec(`
	CREATE TABLE IF NOT EXISTS bookmark_deleted (
		url TEXT PRIMARY KEY,
		deleted_at TEXT
	);
	`)
	return err
}

// migrateV5toV6 adds the bookmark_deleted tombstone table.
func (r *Database) migrateV5toV6() error {
	if err := r.createDeletedTable(); err != nil {
		return err
	}
	if err := r.setUserVersion(6); err != nil {
		return err
	}
	r.version = 6
	log.Println("Database migrated to v6")
	return nil
}

// createStatusTable creates the sibling table that stores HTTP status codes
// per URL. Kept outside the FTS5 table since FTS5 doesn't support ALTER TABLE.
func (r *Database) createStatusTable() error {
	_, err := r.db.Exec(`
	CREATE TABLE IF NOT EXISTS bookmark_status (
		url TEXT PRIMARY KEY,
		status_code INTEGER,
		last_checked TEXT
	);
	`)
	return err
}

// migrateV4toV5 re-normalises every URL through the (possibly updated)
// NormalizeURL function and merges any resulting collisions. Idempotent —
// rows already in canonical form are left alone, and re-running is a no-op
// once swept (the user_version bump ensures it).
//
// Merge rule on collision: prefer the row with non-empty text. If both have
// text, keep the existing canonical row and drop the duplicate. The status
// entry follows the surviving row.
func (r *Database) migrateV4toV5() error {
	type rename struct{ oldURL, newURL string }
	rows, err := r.db.Query("SELECT url FROM bookmarks")
	if err != nil {
		return err
	}
	var todo []rename
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			_ = rows.Close()
			return err
		}
		n := NormalizeURL(u)
		if n != u {
			todo = append(todo, rename{u, n})
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}

	var renamed, merged int
	for _, t := range todo {
		existing, _ := r.GetByUrl(t.newURL)
		if existing == nil {
			if _, err := r.db.Exec("UPDATE bookmarks SET url = ? WHERE url = ?", t.newURL, t.oldURL); err != nil {
				return err
			}
			if _, err := r.db.Exec("UPDATE bookmark_status SET url = ? WHERE url = ?", t.newURL, t.oldURL); err != nil {
				return err
			}
			renamed++
			continue
		}
		// Collision: merge the old row into the canonical one.
		old, err := r.GetByUrl(t.oldURL)
		if err != nil {
			return err
		}
		if (existing.Text == "" || !existing.IsScraped.Bool) && old.Text != "" {
			if _, err := r.db.Exec(
				`UPDATE bookmarks SET text = ?, content_type = ?, isScraped = ? WHERE url = ?`,
				old.Text, old.ContentType, old.IsScraped, t.newURL,
			); err != nil {
				return err
			}
		}
		// Move status to the canonical row only if it doesn't already have one.
		if _, err := r.db.Exec(
			`INSERT OR IGNORE INTO bookmark_status (url, status_code, last_checked)
			 SELECT ?, status_code, last_checked FROM bookmark_status WHERE url = ?`,
			t.newURL, t.oldURL,
		); err != nil {
			return err
		}
		if _, err := r.db.Exec("DELETE FROM bookmark_status WHERE url = ?", t.oldURL); err != nil {
			return err
		}
		if _, err := r.db.Exec("DELETE FROM bookmarks WHERE url = ?", t.oldURL); err != nil {
			return err
		}
		merged++
	}
	if err := r.setUserVersion(5); err != nil {
		return err
	}
	r.version = 5
	if renamed > 0 || merged > 0 {
		log.Printf("Database migrated to v5 (%d renamed, %d merged)", renamed, merged)
	} else {
		log.Println("Database migrated to v5 (no changes)")
	}
	return nil
}

// migrateV3toV4 adds the bookmark_status sibling table.
func (r *Database) migrateV3toV4() error {
	if err := r.createStatusTable(); err != nil {
		return err
	}
	if err := r.setUserVersion(4); err != nil {
		return err
	}
	r.version = 4
	log.Println("Database migrated to v4")
	return nil
}

func (r *Database) createV2() error {
	_, err := r.db.Exec(`
	CREATE VIRTUAL TABLE bookmarks USING FTS5(
	    url,
	    title,
	    text,
	    content_type,
	    isScraped UNINDEXED,
	    dateAdded UNINDEXED,
	    tokenize = 'porter unicode61 remove_diacritics 1'
	);
	`)
	return err
}

// migrateV6toV7 rebuilds the bookmarks FTS5 table with the porter+unicode61
// tokenizer so searches stem morphological variants ("design" matches
// "designed", "designs", "designing"). The table is rebuilt by copying rows
// into a new virtual table and renaming — FTS5 doesn't support ALTER on
// tokenizer.
func (r *Database) migrateV6toV7() error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`
	CREATE VIRTUAL TABLE bookmarks_v7 USING FTS5(
	    url,
	    title,
	    text,
	    content_type,
	    isScraped UNINDEXED,
	    dateAdded UNINDEXED,
	    tokenize = 'porter unicode61 remove_diacritics 1'
	);`); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO bookmarks_v7 (url, title, text, content_type, isScraped, dateAdded)
		 SELECT url, title, text, content_type, isScraped, dateAdded FROM bookmarks`,
	); err != nil {
		return err
	}
	if _, err := tx.Exec("DROP TABLE bookmarks"); err != nil {
		return err
	}
	if _, err := tx.Exec("ALTER TABLE bookmarks_v7 RENAME TO bookmarks"); err != nil {
		return err
	}
	if _, err := tx.Exec("PRAGMA user_version = 7"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.version = 7
	log.Println("Database migrated to v7 (porter stemming)")
	return nil
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

// ErrDeleted indicates a soft-deleted URL is being re-added; the caller
// should treat it as a no-op rather than an error.
var ErrDeleted = errors.New("bookmark is soft-deleted")

func (r *Database) CreateOrGet(bm Bookmark) (*Bookmark, error) {
	bm.URL = NormalizeURL(bm.URL)

	if r.IsDeleted(bm.URL) {
		return nil, ErrDeleted
	}

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
		if _, err := r.CreateOrGet(bm); err != nil && !errors.Is(err, ErrDeleted) {
			log.Println("CreateMany error for", bm.URL, ":", err)
		}
	}
}

func (r *Database) All(keywords string) ([]Bookmark, error) {
	return r.AllFiltered(keywords, "")
}

// StatusCounts holds the number of bookmarks in each HTTP status bucket plus
// "none" for rows that have never been probed.
type StatusCounts struct {
	S2xx int
	S3xx int
	S4xx int
	S5xx int
	None int
}

// AllFiltered behaves like All but restricts results to a status bucket
// ("2xx", "3xx", "4xx", "5xx", "none", or "" for no filter).
func (r *Database) AllFiltered(keywords, statusFilter string) ([]Bookmark, error) {
	if r.version >= 2 {
		return r.allV2Filtered(keywords, statusFilter)
	}
	return r.allV1(keywords)
}

// CountStatuses returns a count per status bucket across all bookmarks.
// Empty StatusCounts on schemas < v4.
func (r *Database) CountStatuses() (StatusCounts, error) {
	var c StatusCounts
	if r.version < 4 {
		return c, nil
	}
	rows, err := r.db.Query(`
		SELECT
		  CASE
		    WHEN status_code BETWEEN 200 AND 299 THEN '2xx'
		    WHEN status_code BETWEEN 300 AND 399 THEN '3xx'
		    WHEN status_code BETWEEN 400 AND 499 THEN '4xx'
		    WHEN status_code BETWEEN 500 AND 599 THEN '5xx'
		    ELSE 'other'
		  END AS bucket,
		  count(*)
		FROM bookmark_status
		GROUP BY bucket`)
	if err != nil {
		return c, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var n int
		if err := rows.Scan(&bucket, &n); err != nil {
			return c, err
		}
		switch bucket {
		case "2xx":
			c.S2xx = n
		case "3xx":
			c.S3xx = n
		case "4xx":
			c.S4xx = n
		case "5xx":
			c.S5xx = n
		}
	}
	if err := r.db.QueryRow(
		`SELECT count(*) FROM bookmarks WHERE url NOT IN (SELECT url FROM bookmark_status)`,
	).Scan(&c.None); err != nil {
		return c, err
	}
	return c, nil
}

func statusFilterClause(bucket string) (clause string, ok bool) {
	switch bucket {
	case "2xx":
		return ` AND url IN (SELECT url FROM bookmark_status WHERE status_code BETWEEN 200 AND 299)`, true
	case "3xx":
		return ` AND url IN (SELECT url FROM bookmark_status WHERE status_code BETWEEN 300 AND 399)`, true
	case "4xx":
		return ` AND url IN (SELECT url FROM bookmark_status WHERE status_code BETWEEN 400 AND 499)`, true
	case "5xx":
		return ` AND url IN (SELECT url FROM bookmark_status WHERE status_code BETWEEN 500 AND 599)`, true
	case "none":
		return ` AND url NOT IN (SELECT url FROM bookmark_status)`, true
	}
	return "", false
}

func (r *Database) allV2(keywords string) ([]Bookmark, error) {
	return r.allV2Filtered(keywords, "")
}

// sanitizeFTSQuery replaces characters the FTS5 query parser treats as
// punctuation/operators with spaces so user input like "lidarr.audio" or
// "github.com/foo" tokenises into searchable terms (lidarr AND audio).
// Quotes are stripped to avoid unterminated phrase-query errors.
func sanitizeFTSQuery(q string) string {
	var b strings.Builder
	b.Grow(len(q))
	for _, r := range q {
		switch r {
		case '.', '/', ':', '?', '#', '&', '=', '+', '%', '\\', '"', '\'', '(', ')', '[', ']', '{', '}', '<', '>', ',', ';', '!', '@', '|', '~', '`', '^', '*':
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func (r *Database) allV2Filtered(keywords, statusFilter string) ([]Bookmark, error) {
	var rows *sql.Rows
	var err error
	filterClause, _ := statusFilterClause(statusFilter)
	if keywords != "" {
		keywords = sanitizeFTSQuery(keywords)
	}
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
			WHERE text is not '' AND bookmarks MATCH ?`+filterClause+`
			ORDER BY bm25(bookmarks, 1.0, 3.0, 1.0, 1.0, 1.0, 1.0)
			LIMIT ?`,
			"\x02", "\x03", keywords, 50,
		)
	} else {
		rows, err = r.db.Query(
			`SELECT url, title, text, content_type, isScraped, dateAdded
			 FROM bookmarks
			 WHERE text is not ''` + filterClause + `
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
	if err := r.attachStatuses(all); err != nil {
		log.Println("attachStatuses error:", err)
	}
	log.Println(len(all), sander.Pluralize("result", len(all)))
	return all, nil
}

// attachStatuses fills StatusCode on each bookmark from the sibling table.
// No-op on schemas < v4.
func (r *Database) attachStatuses(bms []Bookmark) error {
	if r.version < 4 || len(bms) == 0 {
		return nil
	}
	placeholders := make([]string, len(bms))
	args := make([]any, len(bms))
	for i, bm := range bms {
		placeholders[i] = "?"
		args[i] = bm.URL
	}
	rows, err := r.db.Query(
		"SELECT url, status_code FROM bookmark_status WHERE url IN ("+strings.Join(placeholders, ",")+")",
		args...,
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	byURL := make(map[string]sql.NullInt64, len(bms))
	for rows.Next() {
		var u string
		var code sql.NullInt64
		if err := rows.Scan(&u, &code); err != nil {
			return err
		}
		byURL[u] = code
	}
	for i := range bms {
		if code, ok := byURL[bms[i].URL]; ok {
			bms[i].StatusCode = code
		}
	}
	return nil
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
			WHERE b.text is not '' AND bookmarks MATCH ?
			ORDER BY bm25(bookmarks, 1.0, 3.0, 1.0, 1.0, 1.0)
			LIMIT ?`,
			"\x02", "\x03", keywords, 50,
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
		return nil, err
	}
	return &bm, nil
}

func (r *Database) GetLastDateAddedMicro() (int64, error) {
	row := r.db.QueryRow("SELECT dateAdded FROM bookmarks ORDER BY dateAdded DESC LIMIT 1")

	var bm Bookmark
	if err := row.Scan(&bm.DateAdded); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 100000, nil
		}
		return 0, err
	}
	st := time.Time(bm.DateAdded)
	return st.UnixMicro(), nil
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

// AllURLs returns every bookmark URL. Used by background maintenance jobs
// (e.g. status refresh).
func (r *Database) AllURLs() ([]string, error) {
	rows, err := r.db.Query("SELECT url FROM bookmarks")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

// ClearStatuses removes every row from bookmark_status so a refresh pass can
// repopulate them. No-op on schema < v4.
func (r *Database) ClearStatuses() error {
	if r.version < 4 {
		return nil
	}
	_, err := r.db.Exec("DELETE FROM bookmark_status")
	return err
}

// GetText returns the full extracted text for a URL. Empty string if the row
// has no text or doesn't exist.
func (r *Database) GetText(url string) (string, error) {
	var t string
	err := r.db.QueryRow("SELECT text FROM bookmarks WHERE url = ?", url).Scan(&t)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return t, nil
}

// IsDeleted reports whether a URL is in the tombstone table.
func (r *Database) IsDeleted(url string) bool {
	if r.version < 6 {
		return false
	}
	var one int
	err := r.db.QueryRow("SELECT 1 FROM bookmark_deleted WHERE url = ?", url).Scan(&one)
	return err == nil
}

// SoftDelete removes the bookmark from the searchable set and records its URL
// in the tombstone table so future imports skip it. Idempotent: deleting an
// already-tombstoned URL is fine.
func (r *Database) SoftDelete(url string) error {
	if r.version < 6 {
		return errors.New("schema does not support soft delete")
	}
	if err := r.Delete(url); err != nil && !errors.Is(err, ErrDeleteFailed) {
		return err
	}
	_, err := r.db.Exec(
		`INSERT INTO bookmark_deleted (url, deleted_at) VALUES (?, ?)
		 ON CONFLICT(url) DO UPDATE SET deleted_at = excluded.deleted_at`,
		url, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// LookupStatus returns the persisted HTTP status for a URL, or an invalid
// NullInt64 if none is stored (or schema < v4).
func (r *Database) LookupStatus(url string) sql.NullInt64 {
	var code sql.NullInt64
	if r.version < 4 {
		return code
	}
	_ = r.db.QueryRow("SELECT status_code FROM bookmark_status WHERE url = ?", url).Scan(&code)
	return code
}

// UpsertStatus records the latest HTTP status code seen for a URL.
// Stored in the sibling bookmark_status table; safe no-op on schemas < v4.
func (r *Database) UpsertStatus(url string, statusCode int) error {
	if r.version < 4 {
		return nil
	}
	_, err := r.db.Exec(
		`INSERT INTO bookmark_status (url, status_code, last_checked) VALUES (?, ?, ?)
		 ON CONFLICT(url) DO UPDATE SET status_code = excluded.status_code, last_checked = excluded.last_checked`,
		url, statusCode, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// UpdateURL renames a bookmark's URL in place. Returns ErrDuplicate if a row
// with newURL already exists, ErrNotExists if oldURL is missing.
func (r *Database) UpdateURL(oldURL, newURL string) error {
	if oldURL == newURL {
		return nil
	}
	if existing, _ := r.GetByUrl(newURL); existing != nil {
		return ErrDuplicate
	}
	res, err := r.db.Exec("UPDATE bookmarks SET url = ? WHERE url = ?", newURL, oldURL)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotExists
	}
	if r.version < 2 {
		_, _ = r.db.Exec("UPDATE bookmark_meta SET url = ? WHERE url = ?", newURL, oldURL)
	}
	if r.version >= 4 {
		_, _ = r.db.Exec("UPDATE bookmark_status SET url = ? WHERE url = ?", newURL, oldURL)
	}
	return nil
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

	if r.version < 2 {
		_, _ = r.db.Exec("DELETE FROM bookmark_meta WHERE url = ?", url)
	}
	if r.version >= 4 {
		_, _ = r.db.Exec("DELETE FROM bookmark_status WHERE url = ?", url)
	}

	return err
}

var BmDb *Database
