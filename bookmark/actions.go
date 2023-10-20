package bookmark

import (
	"database/sql"
	"fafi2/sander"
	"github.com/advancedlogic/GoOse"
	"log"
)

func Index(bm Bookmark) {
	bmDb := BmDb
	sourceUrl := bm.URL
	defer func() {
		if r := recover(); r != nil {
			// Handle the panic here, you can log the error or take any other necessary action.
			log.Println("Recovered from panic:", r)
			bm.IsScraped = sql.NullBool{Bool: true, Valid: true}
			_, _ = bmDb.Update(sourceUrl, bm)
		}
	}()

	log.Println("Indexing:", sourceUrl)
	g := goose.New()
	article, err := g.ExtractFromURL(sourceUrl)
	if err != nil {
		// Avoid reindexing
		log.Println("Indexing error:", err)
		bm.IsScraped = sql.NullBool{Bool: true, Valid: true}
		_, _ = bmDb.Update(sourceUrl, bm)
		return
	}
	if article.Title != "" {
		bm.Title = article.Title
	} else {
		bm.Title = article.FinalURL
	}
	if bm.Text == "" {
		// heuristic to filter out bookmarks to files
		if len(article.Links) > 0 {
			bm.Text = article.CleanedText
		}
	}
	bm.URL = article.FinalURL
	bm.IsScraped = sql.NullBool{Bool: true, Valid: true}

	// Update
	_, err = bmDb.Update(sourceUrl, bm)
	if err != nil {
		log.Fatal("Update error:", err)
		return
	}
}

func CreateSampleBookmarks(r *Database) {
	skipRecords := sander.GetEnv("FAFI_SKIP_RECORDS", "")
	if skipRecords == "" {
		bms := [2]Bookmark{
			{
				URL: "https://vandragt.com",
			},
			{
				URL: "https://github.com/svandragt/fafi",
			},
		}
		for _, bm := range bms {
			_, err := r.CreateOrGet(bm)
			if err != nil {
				log.Fatal("CreateOrGet sample records error:", err)
			}
		}
	}
}
