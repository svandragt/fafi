package main

import (
	"database/sql"
	"fafi2/bookmark"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
)

type PageData struct {
	Bookmarks []bookmark.Bookmark
	Query     string
}

func main() {
	bootEnvironment()
	db := bootDatabase()
	defer func(db *sql.DB) {
		_ = db.Close()
	}(db)

	bootIndexer()
	// Server start
	bootServer()
}

// Index any bookmarks without text
func bootIndexer() {

	go func() {
		for {
			queue, err := bookmark.BmDb.SelectQueue()
			if len(queue) > 0 {
				log.Printf("Queued %d bookmarks\n", len(queue))
			}
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
				log.Println("updated " + bm.URL)
			}
			if len(queue) > 0 {
				log.Println("Queue emptied")
			}
			return
		}
	}()
}

// bootServer Starts Web Server
func bootServer() {
	port := os.Getenv("FAFI_PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", handleIndex)

	log.Println("Server starting on http://localhost:" + port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("BootServer error: ", err)
	}
}

// Database access
func bootDatabase() *sql.DB {
	fn := "fafi.sqlite3"
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

	if err := bookmark.BmDb.Migrate(); err != nil {
		log.Fatal("Migration error:", err)
	}

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
		log.Println("Search: ", searchQuery)
	}
	return searchQuery
}
