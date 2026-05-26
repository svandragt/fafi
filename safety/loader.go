package safety

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// URLhausFeedURL is the canonical URLhaus "online URLs" CSV. Only currently
// online entries are listed there, so we don't need to filter by status —
// but we still defensively check the column.
const URLhausFeedURL = "https://urlhaus.abuse.ch/downloads/csv_online/"

// maxFeedBytes caps how much we'll read from the feed. URLhaus online is a
// few MB today; 50 MB is comfortable headroom and stops a hostile feed from
// exhausting memory. var so tests can shrink it.
var maxFeedBytes int64 = 50 * 1024 * 1024

// Loader fetches and parses a blocklist feed, persists the raw bytes to disk
// so restarts have data immediately, and exposes the current snapshot via an
// atomically-swapped *Set.
type Loader struct {
	FeedURL    string
	Dir        string
	Interval   time.Duration
	HTTPClient *http.Client
	current    atomic.Pointer[Set]
}

// NewLoader returns a Loader with defaults applied. dir is created on demand
// when Refresh writes the snapshot file.
func NewLoader(feedURL, dir string, refresh time.Duration) *Loader {
	if feedURL == "" {
		feedURL = URLhausFeedURL
	}
	if refresh <= 0 {
		refresh = 6 * time.Hour
	}
	l := &Loader{
		FeedURL:    feedURL,
		Dir:        dir,
		Interval:   refresh,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
	l.current.Store(NewSet())
	return l
}

// Snapshot returns the current in-memory blocklist. Always non-nil.
func (l *Loader) Snapshot() *Set {
	return l.current.Load()
}

// Check is a convenience wrapper around Snapshot().Check.
func (l *Loader) Check(rawURL string) (string, bool) {
	return l.Snapshot().Check(rawURL)
}

// LoadFromDisk populates the in-memory set from a previously-persisted CSV.
// Missing file is not an error — the caller is expected to follow up with a
// Refresh in that case.
func (l *Loader) LoadFromDisk() error {
	path := l.diskPath()
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()
	set, n, err := parseURLhaus(f)
	if err != nil {
		return err
	}
	l.current.Store(set)
	log.Printf("Safety: loaded %d entries from %s", n, path)
	return nil
}

// Refresh downloads the feed, parses it, and on success persists it and
// swaps it in. On any failure the previous snapshot is kept.
func (l *Loader) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.FeedURL, nil)
	if err != nil {
		return err
	}
	resp, err := l.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("feed HTTP %d", resp.StatusCode)
	}

	// Read into a bounded buffer so we can both parse and persist.
	limited := io.LimitReader(resp.Body, maxFeedBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if int64(len(body)) > maxFeedBytes {
		return fmt.Errorf("feed exceeds %d bytes", maxFeedBytes)
	}

	set, n, err := parseURLhaus(strings.NewReader(string(body)))
	if err != nil {
		return err
	}

	if err := l.persist(body); err != nil {
		// Persist failure is not fatal — we still got a fresh snapshot.
		log.Println("Safety: persist error:", err)
	}
	l.current.Store(set)
	log.Printf("Safety: refreshed %d entries from %s", n, l.FeedURL)
	return nil
}

// RunPeriodic refreshes immediately if the on-disk copy is missing or older
// than Interval, then loops on Interval until ctx is cancelled. Failures
// are logged and don't stop the loop — the previous snapshot stays in
// effect.
func (l *Loader) RunPeriodic(ctx context.Context) {
	if l.stale() {
		if err := l.Refresh(ctx); err != nil {
			log.Println("Safety: initial refresh failed:", err)
		}
	}
	t := time.NewTicker(l.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := l.Refresh(ctx); err != nil {
				log.Println("Safety: refresh failed:", err)
			}
		}
	}
}

func (l *Loader) diskPath() string {
	return filepath.Join(l.Dir, "urlhaus.csv")
}

func (l *Loader) stale() bool {
	info, err := os.Stat(l.diskPath())
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) >= l.Interval
}

func (l *Loader) persist(body []byte) error {
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return err
	}
	tmp := l.diskPath() + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, l.diskPath())
}

// parseURLhaus reads a URLhaus "online URLs" CSV and returns a populated Set.
// The format is: lines starting with '#' are comments; data rows are quoted
// CSV with columns id,dateadded,url,url_status,last_online,threat,tags,
// urlhaus_link,reporter.
//
// Returns the parsed set, the count of online URLs added, and an error if
// the input exceeds maxFeedBytes or is malformed.
func parseURLhaus(r io.Reader) (*Set, int, error) {
	limited := io.LimitReader(r, maxFeedBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, 0, err
	}
	if int64(len(data)) > maxFeedBytes {
		return nil, 0, fmt.Errorf("feed exceeds %d bytes", maxFeedBytes)
	}

	set := NewSet()
	var added int

	// Strip comment lines before handing to csv.Reader — Go's CSV parser
	// doesn't have a built-in comment skip for '#' at start of line that
	// also handles the blank "#\n" form, but its Comment field does. Use it.
	cr := csv.NewReader(strings.NewReader(string(data)))
	cr.Comment = '#'
	cr.FieldsPerRecord = -1 // tolerate occasional schema drift
	cr.LazyQuotes = true

	for {
		row, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// Skip a single malformed row rather than aborting the whole feed.
			continue
		}
		if len(row) < 4 {
			continue
		}
		rawURL := strings.TrimSpace(row[2])
		status := strings.ToLower(strings.TrimSpace(row[3]))
		if rawURL == "" || status != "online" {
			continue
		}
		set.AddURL(rawURL)
		added++
	}
	return set, added, nil
}
