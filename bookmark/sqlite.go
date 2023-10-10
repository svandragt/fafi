// Not sure if we need an bm.ID
// Currently there is none, but some functions here implement it
// the URL is the primary key surely to get dedupe for free.
// TODO FIX https://gosamples.dev/sqlite-intro/

package bookmark

import (
	"database/sql"
	"errors"
	"github.com/mattn/go-sqlite3"
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
	    url PRIMARY KEY, 
	    title, 
	    text, 
	    date_added
	);
    `

	_, err := r.db.Exec(query)
	return err
}

func (r *Database) Create(bm Bookmark) (*Bookmark, error) {
	query := `
INSERT INTO bookmarks(url, title, text, date_added) values(?,?,?,?)
`
	res, err := r.db.Exec(query, bm.URL, bm.Title, bm.Text, bm.DateAdded)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) {
			if errors.Is(sqliteErr.ExtendedCode, sqlite3.ErrConstraintUnique) {
				return nil, ErrDuplicate
			}
		}
		return nil, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	bm.ID = id

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
		if err := rows.Scan(&bm.ID, &bm.Name, &bm.URL, &bm.Rank); err != nil {
			return nil, err
		}
		all = append(all, bm)
	}
	return all, nil
}

func (r *Database) GetByName(name string) (*Bookmark, error) {
	row := r.db.QueryRow("SELECT * FROM bookmarks WHERE name = ?", name)

	var bm Bookmark
	if err := row.Scan(&bm.ID, &bm.Name, &bm.URL, &bm.Rank); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotExists
		}
		return nil, err
	}
	return &bm, nil
}
