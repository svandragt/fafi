// Not sure if we need an bm.ID
// Currently there is none, but some functions here implement it
// the URL is the primary key surely to get dedupe for free.
// TODO FIX https://gosamples.dev/sqlite-intro/

package bookmark

import (
	"database/sql"
	"errors"
	"github.com/mattn/go-sqlite3"
	"log"
	"os"
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

func (r *Database) Migrate() error {
	query := `
	CREATE VIRTUAL TABLE bookmarks USING FTS5(
	    url, 
	    title, 
	    text, 
	    date_added
	);
    `
	_, err := r.db.Exec(query)
	if err != nil {
		log.Println("Info:", err)
		return nil
	}

	// TODO:
	if os.Getenv("FAFI_SKIP_RECORDS") == "" {
		bms := [2]Bookmark{
			{
				Title: "test 1",
				URL:   "https://vandragt.com",
			},
			{
				Title: "test 2",
				URL:   "https://github.com/svandragt/fafi",
			},
		}
		for _, bm := range bms {
			_, err := r.Create(bm)
			if err != nil {
				log.Fatal("Create sample records error:", err)
			}
		}
	}
	return err
}

func (r *Database) Create(bm Bookmark) (*Bookmark, error) {
	bm.DateAdded = time.Now()

	query := `
INSERT INTO bookmarks(url, title, text, date_added) values(?,?,?,?)
`
	_, err := r.db.Exec(query, bm.URL, bm.Title, bm.Text, bm.DateAdded)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
				return nil, ErrDuplicate
			}
		}
		return nil, err
	}

	return &bm, nil
}

func (r *Database) All() ([]Bookmark, error) {
	rows, err := r.db.Query("SELECT * FROM bookmarks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []Bookmark
	for rows.Next() {
		var bm Bookmark
		if err := rows.Scan(&bm.Title, &bm.Text, &bm.URL, &bm.DateAdded); err != nil {
			return nil, err
		}
		all = append(all, bm)
	}
	return all, nil
}

func (r *Database) GetByUrl(url string) (*Bookmark, error) {
	row := r.db.QueryRow("SELECT * FROM bookmarks WHERE url = ?", url)

	var bm Bookmark
	if err := row.Scan(&bm.Title, &bm.Text, &bm.URL, &bm.DateAdded); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotExists
		}
		return nil, err
	}
	return &bm, nil
}

func (r *Database) Update(url string, updated Bookmark) (*Bookmark, error) {
	res, err := r.db.Exec(
		"UPDATE bookmarks SET title = ?, text = ? WHERE url = ?",
		updated.Title,
		updated.Text,
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
