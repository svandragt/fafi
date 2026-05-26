package bookmark

import (
	"database/sql"
	"fafi2/sander"
	"github.com/advancedlogic/GoOse"
	"html"
	"log"
	"regexp"
	"strings"
)

func Index(bm Bookmark) {
	index(bm, false)
}

// Reindex re-scrapes the bookmark and overwrites title/text when the
// extracted values are non-empty. Used by the per-result reindex action.
func Reindex(bm Bookmark) {
	index(bm, true)
}

func index(bm Bookmark, overwrite bool) {
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

	ct, status, err := ProbeContentType(sourceUrl)
	if status != 0 {
		if sErr := bmDb.UpsertStatus(sourceUrl, status); sErr != nil {
			log.Println("UpsertStatus error:", sErr)
		}
	} else {
		if sErr := bmDb.RecordFailure(sourceUrl); sErr != nil {
			log.Println("RecordFailure error:", sErr)
		}
	}
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
	if overwrite || bm.Title == "" {
		extracted := pickTitle(article)
		switch {
		case extracted != "":
			bm.Title = extracted
		case overwrite:
			// Reindex of a previously-malformed row — replace with URL so the
			// bad title doesn't linger.
			bm.Title = article.FinalURL
		case bm.Title == "":
			bm.Title = article.FinalURL
		}
	}
	if overwrite {
		if article.CleanedText != "" {
			bm.Text = article.CleanedText
		}
	} else if bm.Text == "" {
		bm.Text = article.CleanedText
	}
	bm.URL = article.FinalURL
	bm.IsScraped = sql.NullBool{Bool: true, Valid: true}

	if _, err := bmDb.Update(sourceUrl, bm); err != nil {
		log.Println("Update error:", err)
	}
}

var titleTagRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// firstHTMLTitle returns the text inside the first <title> tag of the raw
// HTML. Goose's cleaned article.Doc is unreliable here — some pages (e.g.
// theverge.com, newrepublic.com) carry multiple <title> elements and Goose's
// goquery selection concatenates them, producing garbled nav text. Parsing
// the raw HTML directly avoids that.
func firstHTMLTitle(rawHTML string) string {
	m := titleTagRe.FindStringSubmatch(rawHTML)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(m[1]))
}

// pickTitle prefers the first raw <title> tag over Goose's extracted title.
// Falls back to Goose's title only when the <title> tag is empty or itself
// looks malformed. Returns "" when no usable candidate exists so the caller
// can substitute the URL.
func pickTitle(article *goose.Article) string {
	headTitle := firstHTMLTitle(article.RawHTML)
	if headTitle != "" && !looksMalformedTitle(headTitle) {
		return headTitle
	}
	if article.Title != "" && !looksMalformedTitle(article.Title) {
		return article.Title
	}
	if headTitle != "" && len(headTitle) <= 300 {
		return headTitle
	}
	return ""
}

// looksMalformedTitle returns true for strings that are obviously nav/chrome
// pollution rather than a real title: very long, or full of repeated phrases.
//
// The repetition check slides an 8-character window across the (lowercased)
// string and flags it when any window occurs three or more times. That catches
// both "ExpandExpandExpand…" and "The New RepublicThe New Republic…" without
// needing a hard-coded marker list.
func looksMalformedTitle(s string) bool {
	if len(s) > 300 {
		return true
	}
	low := strings.ToLower(s)
	const window = 8
	if len(low) < window*3 {
		return false
	}
	seen := make(map[string]int, len(low))
	for i := 0; i+window <= len(low); i++ {
		w := low[i : i+window]
		seen[w]++
		if seen[w] >= 3 {
			return true
		}
	}
	return false
}

// RefreshAllStatuses probes every bookmark URL and upserts its HTTP status
// without touching extracted text or isScraped. Intended for backfilling
// bookmark_status after schema upgrades or on FAFI_RESET_STATUS=1.
func RefreshAllStatuses(progressInc func()) {
	urls, err := BmDb.AllURLs()
	if err != nil {
		log.Println("RefreshAllStatuses: list error:", err)
		return
	}
	log.Printf("Refreshing HTTP status for %d bookmarks", len(urls))
	const workers = 8
	sem := make(chan struct{}, workers)
	for _, u := range urls {
		sem <- struct{}{}
		go func(u string) {
			defer func() { <-sem }()
			_, status, _ := ProbeContentType(u)
			if status != 0 {
				if err := BmDb.UpsertStatus(u, status); err != nil {
					log.Println("RefreshAllStatuses upsert error:", err)
				}
			} else {
				if err := BmDb.RecordFailure(u); err != nil {
					log.Println("RefreshAllStatuses RecordFailure error:", err)
				}
			}
			if progressInc != nil {
				progressInc()
			}
		}(u)
	}
	// Drain.
	for i := 0; i < workers; i++ {
		sem <- struct{}{}
	}
}

// RefreshMissingStatuses probes only URLs that have no entry in bookmark_status
// and upserts the result. Cheap, idempotent: re-runs are no-ops once every row
// has a status. Used to back-fill rows indexed before status tracking landed.
func RefreshMissingStatuses(progressInc func()) {
	urls, err := BmDb.URLsMissingStatus()
	if err != nil {
		log.Println("RefreshMissingStatuses: list error:", err)
		return
	}
	if len(urls) == 0 {
		return
	}
	log.Printf("Backfilling HTTP status for %d bookmarks", len(urls))
	const workers = 8
	sem := make(chan struct{}, workers)
	for _, u := range urls {
		sem <- struct{}{}
		go func(u string) {
			defer func() { <-sem }()
			_, status, _ := ProbeContentType(u)
			if status != 0 {
				if err := BmDb.UpsertStatus(u, status); err != nil {
					log.Println("RefreshMissingStatuses upsert error:", err)
				}
			} else {
				if err := BmDb.RecordFailure(u); err != nil {
					log.Println("RefreshMissingStatuses RecordFailure error:", err)
				}
			}
			if progressInc != nil {
				progressInc()
			}
		}(u)
	}
	for i := 0; i < workers; i++ {
		sem <- struct{}{}
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
