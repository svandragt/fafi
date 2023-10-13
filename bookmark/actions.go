package bookmark

import (
	"database/sql"
	"github.com/advancedlogic/GoOse"
	"log"
)

func Index(bm Bookmark) {
	sourceUrl := bm.URL
	g := goose.New()
	article, _ := g.ExtractFromURL(sourceUrl)
	if bm.Title == "" {
		bm.Title = article.Title
	}
	if bm.Text == "" {
		bm.Text = article.CleanedText
	}
	bm.URL = article.FinalURL
	bm.IsScraped = sql.NullBool{Bool: true, Valid: true}

	// Update
	bmDb := BmDb
	_, err := bmDb.Update(sourceUrl, bm)
	if err != nil {
		log.Fatal("Update error:", err)
		return
	}
}
