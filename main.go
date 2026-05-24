package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fafi2/bookmark"
	"fafi2/integration"
	"fafi2/progress"
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
	Bookmarks []bookmark.Bookmark
	Query     string
}

//go:embed pub/index.html
var EmbedHtmlIndex string

func main() {
	bootEnvironment()
	sander.UpdateDebugState()
	db := bootDatabase()
	defer func(db *sql.DB) {
		_ = db.Close()
	}(db)

	// Check if --firefox argument was present
	firefoxProfilePath := sander.GetArgFromEnvWithDefault("FAFI_FIREFOX", "")

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

// Environment variables
func bootEnvironment() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Environment error:", err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	keywords := getSearchQuery(w, r)
	bookmarks, err := bookmark.BmDb.All(keywords)
	if err != nil {
		log.Fatal("Bookmark error:", err)
	}

	var tpl = template.Must(template.New("EmbedHtmlIndex").Funcs(template.FuncMap{
		"contentTypeIcon": contentTypeIcon,
	}).Parse(EmbedHtmlIndex))

	data := TemplateData{
		Bookmarks: bookmarks,
		Query:     keywords,
	}
	err = tpl.Execute(w, data)
	if err != nil {
		log.Fatal("Error (execute):", err)
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
