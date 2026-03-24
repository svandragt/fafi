package main

import (
	"fafi2/bookmark"
	"sync"
	"sync/atomic"
	"testing"
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
