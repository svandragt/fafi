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
			log.Println("Recovered from panic:", r)
			bm.IsScraped = sql.NullBool{Bool: true, Valid: true}
			_, _ = bmDb.Update(sourceUrl, bm)
		}
	}()

	log.Println("Indexing:", sourceUrl)

	ct, err := ProbeContentType(sourceUrl)
	if err != nil {
		log.Println("Probe error:", err)
		// Leave isScraped unset so transient failures retry on next run.
		return
	}
	bm.ContentType = ct

	if !IsTextual(ct) {
		// Non-text resource: store as a successful index, skip extraction.
		log.Printf("Non-text (%s): %s", ct, sourceUrl)
		bm.IsScraped = sql.NullBool{Bool: true, Valid: true}
		if _, err := bmDb.Update(sourceUrl, bm); err != nil {
			log.Println("Update error:", err)
		}
		return
	}

	g := goose.New()
	article, err := g.ExtractFromURL(sourceUrl)
	if err != nil {
		log.Println("Indexing error:", err)
		return
	}
	if bm.Title == "" {
		if article.Title != "" {
			bm.Title = article.Title
		} else {
			bm.Title = article.FinalURL
		}
	}
	if bm.Text == "" {
		bm.Text = article.CleanedText
	}
	bm.URL = article.FinalURL
	bm.IsScraped = sql.NullBool{Bool: true, Valid: true}

	if _, err := bmDb.Update(sourceUrl, bm); err != nil {
		log.Println("Update error:", err)
	}
}

func CreateSampleBookmarks(r *Database) {
	skipRecords := sander.GetArgFromEnvWithDefault("FAFI_SKIP_RECORDS", "0")
	if skipRecords == "0" {
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
