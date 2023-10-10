package main

import (
	"database/sql"
	"fafi2/bookmark"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

var bmDb *bookmark.Database

type PageData struct {
	Bookmarks []bookmark.Bookmark
	Query     string
}

func main() {
	bootEnvironment()
	db := bootDatabase()
	defer db.Close()

	// Server start
	bootServer()
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
		log.Fatal("Error:", err)
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

	bmDb = bookmark.NewDatabase(db)

	if err := bmDb.Migrate(); err != nil {
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
	bookmarks, err := bmDb.All()
	if err != nil {
		log.Fatal("Error:", err)
	}
	var tpl = template.Must(template.ParseFiles("pub/index.html"))

	data := PageData{
		Bookmarks: bookmarks,
		Query:     getSearchQuery(w, r),
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
