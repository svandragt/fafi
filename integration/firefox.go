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
	tmpFile, _ := sander.CopyToTmp(path, "fafi-firefox-*.sqlite3")
	defer func(tmpFile **os.File) {
		tmpPath := (*tmpFile).Name()
		log.Println("Deleting tmpfile:", tmpPath)
		_ = os.Remove(tmpPath)
	}(tmpFile)

	var db, err = sql.Open("sqlite3", "file:"+(*tmpFile).Name())
	if err != nil {
		log.Fatal("DB Opening error:", err)
	}

	ffDb := NewDatabase(db)

	//Bookmarks
	bookmarks, err := ffDb.SelectMozBookmarks()
	if err != nil {
		log.Fatal("SelectMozBookmarks error:", err)
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
    SELECT DISTINCT
        url, moz_places.title, dateAdded from moz_places  
    JOIN 
        moz_bookmarks on moz_bookmarks.fk=moz_places.id 
    WHERE 
        moz_places.url like 'http%' and dateAdded >= ?
    ORDER BY 
        dateAdded
`
	lastDateAddedMicro := bookmark.BmDb.GetLastDateAddedMicro()
	rows, err = r.db.Query(query, lastDateAddedMicro)

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
