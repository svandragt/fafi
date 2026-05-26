package main

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fafi2/bookmark"
	"fafi2/integration"
	"fafi2/progress"
	"fafi2/safety"
	"fafi2/sander"
	"fmt"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// contentTypeIcon returns a category icon for a MIME type, or "" for text/HTML
// (no icon needed). Empty input also returns "" — legacy rows without a
// detected type render plainly.
func contentTypeIcon(ct string) string {
	if ct == "" || strings.HasPrefix(ct, "text/") {
		return ""
	}
	switch {
	case strings.HasPrefix(ct, "image/"):
		return "🖼"
	case strings.HasPrefix(ct, "video/"):
		return "🎬"
	case strings.HasPrefix(ct, "audio/"):
		return "🎵"
	case ct == "application/pdf":
		return "📄"
	default:
		return "📦"
	}
}

var indexProgress = progress.New()

type TemplateData struct {
	Bookmarks    []bookmark.Bookmark
	Query        string
	Status       string
	StatusCounts bookmark.StatusCounts
	ResultCount  int
	ElapsedMs    int64
}

//go:embed pub/index.html
var EmbedHtmlIndex string

// highlightSnippet renders a snippet that may contain FTS5 match markers
// (\x02 open, \x03 close) as HTML, wrapping marked runs in <mark>. Truncates
// to n visible runes, ignoring marker bytes so the cap reflects what the
// reader sees. The output is safe for template injection.
func highlightSnippet(s string, n int) template.HTML {
	var b strings.Builder
	b.Grow(len(s))
	visible := 0
	inMark := false
	truncated := false
	for _, r := range s {
		switch r {
		case '\x02':
			if !inMark {
				b.WriteString("<mark>")
				inMark = true
			}
			continue
		case '\x03':
			if inMark {
				b.WriteString("</mark>")
				inMark = false
			}
			continue
		}
		if visible >= n {
			truncated = true
			break
		}
		template.HTMLEscape(&b, []byte(string(r)))
		visible++
	}
	if inMark {
		b.WriteString("</mark>")
	}
	if truncated {
		b.WriteString("…")
	}
	return template.HTML(b.String())
}

// statusClass maps an HTTP status code to a CSS class for the pill color.
func statusClass(code int64) string {
	switch {
	case code >= 200 && code < 300:
		return "s2"
	case code >= 300 && code < 400:
		return "s3"
	case code >= 400 && code < 500:
		return "s4"
	case code >= 500 && code < 600:
		return "s5"
	default:
		return "s0"
	}
}

// filterURL builds the index URL preserving the current query while setting
// (or clearing) the status filter.
func filterURL(query, status string) string {
	v := url.Values{}
	if query != "" {
		v.Set("q", query)
	}
	if status != "" {
		v.Set("status", status)
	}
	if len(v) == 0 {
		return "/"
	}
	return "/?" + v.Encode()
}

// truncateRunes returns s truncated to n runes with an ellipsis if cut.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var indexTpl = template.Must(template.New("EmbedHtmlIndex").Funcs(template.FuncMap{
	"contentTypeIcon":  contentTypeIcon,
	"highlightSnippet": highlightSnippet,
	"statusClass":      statusClass,
	"filterURL":        filterURL,
	"truncateRunes":    truncateRunes,
}).Parse(EmbedHtmlIndex))

func main() {
	bootEnvironment()
	sander.UpdateDebugState()
	db := bootDatabase()
	defer func(db *sql.DB) {
		_ = db.Close()
	}(db)

	// Check if --firefox argument was present
	firefoxProfilePath := sander.GetArgFromEnvWithDefault("FAFI_FIREFOX", "")

	bootSafety()

	go func() {
		if firefoxProfilePath != "" {
			integration.ImportFirefoxProfile(firefoxProfilePath)
		}
		if sander.GetArgFromEnvWithDefault("FAFI_RESET_INDEX", "0") == "1" {
			if err := bookmark.BmDb.ResetIndex(); err != nil {
				log.Println("ResetIndex error:", err)
			} else {
				log.Println("Indexed state cleared on all bookmarks")
			}
		}
		if sander.GetArgFromEnvWithDefault("FAFI_RESET_STATUS", "0") == "1" {
			if err := bookmark.BmDb.ClearStatuses(); err != nil {
				log.Println("ClearStatuses error:", err)
			} else {
				log.Println("Status cleared; refreshing in background")
				bookmark.RefreshAllStatuses(nil)
				log.Println("Status refresh complete")
			}
		}
		if sander.GetArgFromEnvWithDefault("FAFI_BACKFILL_STATUS", "0") == "1" {
			bookmark.RefreshMissingStatuses(nil)
			log.Println("Status backfill complete")
			if n, err := bookmark.BmDb.AutoSoftDeleteStaleUnreachable(); err != nil {
				log.Println("AutoSoftDelete error:", err)
			} else if n > 0 {
				log.Printf("Soft-deleted %d stale unreachable bookmark(s)", n)
			}
		}
		if sander.GetArgFromEnvWithDefault("FAFI_PURGE_UNREACHABLE", "0") == "1" {
			n, err := bookmark.BmDb.PurgeUnreachable()
			if err != nil {
				log.Println("PurgeUnreachable error:", err)
			} else {
				log.Printf("Purge unreachable: soft-deleted %d bookmark(s) with no text", n)
			}
		}
		enableIndexing := sander.GetArgFromEnvWithDefault("FAFI_ENABLE_INDEXING", "1")
		if enableIndexing == "1" {
			bootIndexer()
		} else {
			log.Println("Indexing skipped")
		}
		log.Println("Ready")
	}()

	bootServer()
}

// indexQueue runs indexFn concurrently for each bookmark in queue,
// bounded to maxIndexConcurrency workers to avoid resource exhaustion.
const maxIndexConcurrency = 8

func indexQueue(queue []bookmark.Bookmark, indexFn func(bookmark.Bookmark)) {
	sem := make(chan struct{}, maxIndexConcurrency)
	var wg sync.WaitGroup
	for _, bm := range queue {
		wg.Add(1)
		sem <- struct{}{}
		go func(bm bookmark.Bookmark) {
			defer wg.Done()
			defer func() { <-sem }()
			indexFn(bm)
		}(bm)
	}
	wg.Wait()
}

// Index any bookmarks without text
func bootIndexer() {
	queue, err := bookmark.BmDb.SelectQueue()
	log.Printf("Indexing queue has %d bookmarks\n", len(queue))
	if err != nil {
		log.Println("Queue init error:", err)
		return
	}
	indexProgress.Start(len(queue))
	defer indexProgress.Finish()
	indexQueue(queue, func(bm bookmark.Bookmark) {
		bookmark.Index(bm)
		indexProgress.Inc()
		log.Println("Indexed " + bm.URL)
	})
}

// bootServer Starts Web Server
func bootServer() {
	port := sander.GetArgFromEnvWithDefault("FAFI_PORT", "8000")

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/events", handleEvents)
	http.HandleFunc("/reindex", handleReindex)
	http.HandleFunc("/edit", handleEdit)
	http.HandleFunc("/delete", handleDelete)
	http.HandleFunc("/text", handleText)
	http.HandleFunc("/note", handleNote)

	log.Println("Server starting on http://localhost:" + port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("BootServer error: ", err)
	}
}

// Database access
func bootDatabase() *sql.DB {
	dbPath := sander.GetArgFromEnvWithDefault("FAFI_DB_FILEPATH", "fafi.sqlite3")
	fn := filepath.Clean(dbPath)
	log.Println("Using " + fn)
	db, err := sql.Open("sqlite3", "file:"+fn)
	if err != nil {
		log.Fatal("DB Opening error:", err)
	}

	var version string
	err = db.QueryRow("SELECT SQLITE_VERSION()").Scan(&version)
	if err != nil {
		log.Fatal("SQLite error:", err)
	}
	log.Println("SQLite3 version", version)

	bookmark.BmDb = bookmark.NewDatabase(db)

	if err := bookmark.BmDb.MigrateSchema(); err != nil {
		log.Fatal("Migration error:", err)
	}
	bookmark.CreateSampleBookmarks(bookmark.BmDb)

	return db
}

// bootSafety wires the URLhaus blocklist. On by default; disable with
// FAFI_BLOCKLIST_ENABLED=0. The on-disk snapshot is reused across restarts so
// the very first request after boot still gets a match.
func bootSafety() {
	if sander.GetArgFromEnvWithDefault("FAFI_BLOCKLIST_ENABLED", "1") != "1" {
		log.Println("Safety: blocklist disabled")
		return
	}
	dir := sander.GetArgFromEnvWithDefault("FAFI_BLOCKLIST_DIR", "blocklists")
	refreshStr := sander.GetArgFromEnvWithDefault("FAFI_BLOCKLIST_REFRESH", "6h")
	refresh, err := time.ParseDuration(refreshStr)
	if err != nil {
		log.Printf("Safety: invalid FAFI_BLOCKLIST_REFRESH %q, using 6h: %v", refreshStr, err)
		refresh = 6 * time.Hour
	}
	loader := safety.NewLoader("", dir, refresh)
	if err := loader.LoadFromDisk(); err != nil {
		log.Println("Safety: load from disk error:", err)
	}
	bookmark.Checker = loader
	go loader.RunPeriodic(context.Background())
}

// Environment variables
func bootEnvironment() {
	if err := godotenv.Load(); err != nil {
		// .env is optional; absence or read errors should not block startup.
		log.Println("No .env loaded:", err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	keywords := getSearchQuery(w, r)
	statusFilter := r.URL.Query().Get("status")
	start := time.Now()
	bookmarks, err := bookmark.BmDb.AllFiltered(keywords, statusFilter)
	elapsed := time.Since(start)
	if err != nil {
		// A malformed FTS query in ?q= can cause this; don't take the
		// server down with the request.
		log.Println("Bookmark query error:", err)
		http.Error(w, "search error", http.StatusBadRequest)
		return
	}

	counts, err := bookmark.BmDb.CountStatuses()
	if err != nil {
		log.Println("CountStatuses error:", err)
	}

	data := TemplateData{
		Bookmarks:    bookmarks,
		Query:        keywords,
		Status:       statusFilter,
		StatusCounts: counts,
		ResultCount:  len(bookmarks),
		ElapsedMs:    elapsed.Milliseconds(),
	}
	if err := indexTpl.Execute(w, data); err != nil {
		log.Println("Template execute error:", err)
	}
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := indexProgress.Subscribe()
	defer indexProgress.Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case state, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(state)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func handleReindex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target := r.FormValue("url")
	if target == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}
	bm, err := bookmark.BmDb.GetByUrl(target)
	if err != nil {
		http.Error(w, "bookmark not found", http.StatusNotFound)
		return
	}
	bookmark.Reindex(*bm)
	// Re-fetch to capture any URL change (goose follows redirects) and the
	// fresh fields. The row may now live at the redirected URL.
	updated, err := bookmark.BmDb.GetByUrl(target)
	if err != nil {
		// URL was rewritten by the indexer; we don't currently track that
		// in a way we can resolve here. Fall back to whatever we have.
		updated = bm
	}
	statusCode := bookmark.BmDb.LookupStatus(updated.URL)

	resp := struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		ContentType string `json:"content_type"`
		Indexed     bool   `json:"indexed"`
		StatusCode  int64  `json:"status_code,omitempty"`
		HasStatus   bool   `json:"has_status"`
	}{
		URL:         updated.URL,
		Title:       updated.Title,
		ContentType: updated.ContentType,
		Indexed:     updated.IsScraped.Valid && updated.IsScraped.Bool,
		HasStatus:   statusCode.Valid,
	}
	if statusCode.Valid {
		resp.StatusCode = statusCode.Int64
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Println("reindex json encode error:", err)
	}
}

func handleEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	oldURL := r.FormValue("url")
	newURL := bookmark.NormalizeURL(r.FormValue("new_url"))
	if oldURL == "" || newURL == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}
	if err := bookmark.BmDb.UpdateURL(oldURL, newURL); err != nil {
		log.Println("UpdateURL error:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	redirectBack(w, r)
}

func handleText(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}
	text, err := bookmark.BmDb.GetText(target)
	if err != nil {
		log.Println("GetText error:", err)
		http.Error(w, "lookup error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if _, err := w.Write([]byte(text)); err != nil {
		log.Println("text write error:", err)
	}
}

func handleNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target := r.FormValue("url")
	if target == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}
	note := r.FormValue("note")
	if err := bookmark.BmDb.UpsertNote(target, note); err != nil {
		log.Println("UpsertNote error:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	saved, _ := bookmark.BmDb.GetNote(target)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(struct {
		URL  string `json:"url"`
		Note string `json:"note"`
	}{URL: target, Note: saved}); err != nil {
		log.Println("note json encode error:", err)
	}
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target := r.FormValue("url")
	if target == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}
	if err := bookmark.BmDb.SoftDelete(target); err != nil {
		log.Println("SoftDelete error:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
		log.Println("delete write error:", err)
	}
}

func redirectBack(w http.ResponseWriter, r *http.Request) {
	ref := r.Referer()
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func getSearchQuery(w http.ResponseWriter, r *http.Request) string {
	u, err := url.Parse(r.URL.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return ""
	}

	params := u.Query()
	searchQuery := params.Get("q")
	if searchQuery != "" {
		log.Println("Searched for: ", searchQuery)
	}
	return searchQuery
}
