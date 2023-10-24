package bookmark

import (
	"database/sql"
	"errors"
	"fafi2/sander"
	"github.com/mattn/go-sqlite3"
	"log"
	"time"
)

var (
	ErrDuplicate    = errors.New("record already exists")
	ErrNotExists    = errors.New("row not exists")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
)

type Database struct {
	db *sql.DB
}

func NewDatabase(db *sql.DB) *Database {
	return &Database{
		db: db,
	}
}

func (r *Database) CreateTable() error {
	query := `
	CREATE VIRTUAL TABLE bookmarks USING FTS5(
	    url, 
	    title, 
	    text,
	    isScraped,
	    dateAdded
	);
    `
	_, err := r.db.Exec(query)
	if err != nil {
		log.Println("Info:", err)
		return nil
	}

	log.Println("Database created")
	return err
}

func (r *Database) CreateOrGet(bm Bookmark) (*Bookmark, error) {

	existingBookmark, err := BmDb.GetByUrl(bm.URL)
	if existingBookmark != nil {
		return existingBookmark, err
	}
	bm.DateAdded = SqlTime(time.Now())

	query := `
INSERT INTO bookmarks (url, title, text, dateAdded) VALUES (?, ?, ?, ?);
`
	_, err = r.db.Exec(query, bm.URL, bm.Title, bm.Text, bm.DateAdded.String())
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
	query := "SELECT * FROM bookmarks WHERE text is not '' ORDER BY dateAdded DESC, title LIMIT 50"

	var err error
	var rows *sql.Rows
	// handle search
	if keywords != "" {
		//goland:noinspection SqlSignature,SqlResolve
		query = `
			SELECT 
                url, 
                title,
                snippet(bookmarks, 2,?,?, '...',64) as text,
                isScraped,
                dateAdded
            FROM 
                bookmarks 
            WHERE 
                text is not '' AND (
                title MATCH ? OR
                url MATCH ? OR
                text MATCH ?
                )
            ORDER BY 
                rank 
            LIMIT ?
`
		rows, err = r.db.Query(query, "[", "]", keywords, keywords, keywords, 50)
	} else {
		rows, err = r.db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			return
		}
	}(rows)

	var all []Bookmark
	for rows.Next() {
		var bm Bookmark
		if err := rows.Scan(&bm.URL, &bm.Title, &bm.Text, &bm.IsScraped, &bm.DateAdded); err != nil {
			return nil, err
		}
		all = append(all, bm)
	}

	log.Println(
		len(all),
		sander.Pluralize("result", len(all)),
	)

	return all, nil
}

func (r *Database) SelectQueue() ([]Bookmark, error) {
	rows, err := r.db.Query("SELECT * FROM bookmarks where isScraped is not 1 ORDER BY RANDOM()")
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			return
		}
	}(rows)

	var all []Bookmark
	for rows.Next() {
		var bm Bookmark
		if err := rows.Scan(&bm.URL, &bm.Title, &bm.Text, &bm.IsScraped, &bm.DateAdded); err != nil {
			if !bm.IsScraped.Valid {
				return all, nil
			}
			return nil, err
		}
		all = append(all, bm)
	}
	return all, nil
}

func (r *Database) GetByUrl(url string) (*Bookmark, error) {
	row := r.db.QueryRow("SELECT * FROM bookmarks WHERE url = ?", url)

	var bm Bookmark
	if err := row.Scan(&bm.URL, &bm.Title, &bm.Text, &bm.IsScraped, &bm.DateAdded); err != nil {
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
	res, err := r.db.Exec(
		"UPDATE bookmarks SET title = ?, text = ?, isScraped = ? WHERE url = ?",
		updated.Title,
		updated.Text,
		updated.IsScraped,
		url,
	)
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

	return err
}

var BmDb *Database
