package bookmark

import (
	"database/sql"
	"github.com/advancedlogic/GoOse"
	"log"
)

func Index(bm Bookmark) {
	// TODO handle 400+ http status codes
	// TODO prevent reindexing the same bookmark next session
	defer func() {
		if r := recover(); r != nil {
			// Handle the panic here, you can log the error or take any other necessary action.
			log.Println("Recovered from panic:", r)
		}
	}()

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
