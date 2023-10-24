package main

import (
	"database/sql"
	"fafi2/bookmark"
	"fafi2/integration"
	"fafi2/sander"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
)

type PageData struct {
	Bookmarks []bookmark.Bookmark
	Query     string
}

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

// Index any bookmarks without text
func bootIndexer() {
	for {
		queue, err := bookmark.BmDb.SelectQueue()
		log.Printf("Indexing queue has %d bookmarks\n", len(queue))
		if err != nil {
			log.Println("Queue init error:", err)
			return
		}
		for _, bm := range queue {
			bookmark.Index(bm)
			if err != nil {
				log.Println("Indexer error:", err)
				continue
			}
			log.Println("Indexed " + bm.URL)
		}
		return
	}
}

// bootServer Starts Web Server
func bootServer() {
	port := sander.GetArgFromEnvWithDefault("FAFI_PORT", "8000")

	http.HandleFunc("/", handleIndex)

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

	if err := bookmark.BmDb.CreateTable(); err != nil {
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

	// TODO: embed templates
	var tpl = template.Must(template.ParseFiles("pub/index.html"))

	data := PageData{
		Bookmarks: bookmarks,
		Query:     keywords,
	}
	err = tpl.Execute(w, data)
	if err != nil {
		log.Fatal("Error (execute):", err)
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
