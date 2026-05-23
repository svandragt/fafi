package integration

import (
	"database/sql"
	"fafi2/bookmark"
	"fafi2/sander"
	"log"
	"os"
	"time"
)

func ImportFirefoxProfile(path string) {
	log.Println("Using " + path)
	tmpFile, err := sander.CopyToTmp(path, "fafi-firefox-*.sqlite3")
	if err != nil {
		log.Println("Firefox import skipped, copy failed:", err)
		return
	}
	defer func(tmpFile **os.File) {
		tmpPath := (*tmpFile).Name()
		log.Println("Deleting tmpfile:", tmpPath)
		_ = os.Remove(tmpPath)
	}(tmpFile)

	db, err := sql.Open("sqlite3", "file:"+(*tmpFile).Name())
	if err != nil {
		log.Println("Firefox import skipped, DB open failed:", err)
		return
	}

	ffDb := NewDatabase(db)

	bookmarks, err := ffDb.SelectMozBookmarks()
	if err != nil {
		log.Println("Firefox import skipped, SelectMozBookmarks failed:", err)
		return
	}

	bookmark.BmDb.CreateMany(bookmarks)
}

type Database struct {
	db *sql.DB
}

func NewDatabase(db *sql.DB) *Database {
	return &Database{
		db: db,
	}
}

func (r *Database) SelectMozBookmarks() ([]bookmark.Bookmark, error) {
	var err error
	var rows *sql.Rows

	query := `
WITH RECURSIVE unfiled(id) AS (
  SELECT id FROM moz_bookmarks WHERE guid='unfiled_____'
  UNION ALL
  SELECT b.id FROM moz_bookmarks b JOIN unfiled u ON b.parent = u.id
)
SELECT DISTINCT
  url, moz_places.title, moz_bookmarks.dateAdded
FROM moz_places
JOIN moz_bookmarks ON moz_bookmarks.fk = moz_places.id
WHERE
  moz_places.url LIKE 'http%'
  AND moz_bookmarks.id NOT IN (SELECT id FROM unfiled)
ORDER BY
  moz_bookmarks.dateAdded
`
	rows, err = r.db.Query(query)

	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			return
		}
	}(rows)

	var bms []bookmark.Bookmark
	for rows.Next() {
		var timestamp int64
		var title sql.NullString
		var bm bookmark.Bookmark
		if err := rows.Scan(&bm.URL, &title, &timestamp); err != nil {
			return nil, err
		}
		bm.DateAdded = bookmark.SqlTime(time.Unix(timestamp, 0))
		if title.Valid {
			bm.Title = title.String
		}
		bms = append(bms, bm)
	}

	log.Printf("Found %d new Firefox bookmarks\n", len(bms))
	return bms, nil
}
