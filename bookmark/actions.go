package bookmark

import (
	"database/sql"
	"github.com/advancedlogic/GoOse"
	"log"
	"os"
)

func Index(bm Bookmark) {
	bmDb := BmDb
	defer func() {
		if r := recover(); r != nil {
			// Handle the panic here, you can log the error or take any other necessary action.
			log.Println("Recovered from panic:", r)
		}
	}()

	sourceUrl := bm.URL
	log.Println("Indexing:", sourceUrl)
	g := goose.New()
	article, err := g.ExtractFromURL(sourceUrl)
	if err != nil {
		// Avoid reindexing
		log.Println("Indexing error:", err)
		bm.IsScraped = sql.NullBool{Bool: true, Valid: true}
		_, err = bmDb.Update(sourceUrl, bm)
		if err != nil {
			log.Fatal("Update error:", err)
			return
		}
		return
	}
	if article.Title != "" {
		bm.Title = article.Title
	}
	if bm.Text == "" {
		bm.Text = article.CleanedText
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
	if os.Getenv("FAFI_SKIP_RECORDS") == "" {
		bms := [2]Bookmark{
			{
				URL: "https://vandragt.com",
			},
			{
				URL: "https://github.com/svandragt/fafi",
			},
		}
		for _, bm := range bms {
			_, err := r.Create(bm)
			if err != nil {
				log.Fatal("Create sample records error:", err)
			}
		}
		log.Println("Sample records created")
	}
}
