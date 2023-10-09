package main

import (
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

var tpl = template.Must(template.ParseFiles("pub/index.html"))

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tpl.Execute(w, nil)
	_ = getSearchQuery(w, r)
}

func getSearchQuery(w http.ResponseWriter, r *http.Request) string {
	u, err := url.Parse(r.URL.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return ""
	}

	params := u.Query()
	searchQuery := params.Get("q")
	log.Println("Search: ", searchQuery)
	return searchQuery
}

func startServer(port string) {
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", indexHandler)

	log.Println("Server starting on http://localhost:" + port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("Error:", err)
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file!")
	}
	port := os.Getenv("FAFI_PORT")

	startServer(port)
}
