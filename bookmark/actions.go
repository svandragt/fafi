package bookmark

import (
	"database/sql"
	"github.com/advancedlogic/GoOse"
	"log"
)

func Index(bm Bookmark) {
	sourceUrl := bm.URL
	g := goose.New()
	article, err := g.ExtractFromURL(sourceUrl)
	if err != nil {
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
	bmDb := BmDb
	_, err = bmDb.Update(sourceUrl, bm)
	if err != nil {
		log.Fatal("Update error:", err)
		return
	}
}
