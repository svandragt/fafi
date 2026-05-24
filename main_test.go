package main

import (
	"database/sql"
	"fafi2/bookmark"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestIndexQueueCallsAllBookmarks(t *testing.T) {
	queue := []bookmark.Bookmark{
		{URL: "http://example.com/1"},
		{URL: "http://example.com/2"},
		{URL: "http://example.com/3"},
	}

	var count int64
	var mu sync.Mutex
	seen := map[string]bool{}

	indexQueue(queue, func(bm bookmark.Bookmark) {
		atomic.AddInt64(&count, 1)
		mu.Lock()
		seen[bm.URL] = true
		mu.Unlock()
	})

	if int(count) != len(queue) {
		t.Errorf("expected %d calls, got %d", len(queue), count)
	}
	for _, bm := range queue {
		if !seen[bm.URL] {
			t.Errorf("bookmark %s was not indexed", bm.URL)
		}
	}
}

// Regression: a DB error from handleIndex must not crash the server.
// Previously handleIndex called log.Fatal on BmDb.All errors. We force
// an error by closing the DB before the request.
func TestHandleIndexDoesNotFatalOnDBError(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	prev := bookmark.BmDb
	bookmark.BmDb = bookmark.NewDatabase(db)
	t.Cleanup(func() { bookmark.BmDb = prev })
	if err := bookmark.BmDb.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema: %v", err)
	}
	_ = db.Close()

	req := httptest.NewRequest(http.MethodGet, "/?q=foo", nil)
	rec := httptest.NewRecorder()
	handleIndex(rec, req) // must not call log.Fatal / panic

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
