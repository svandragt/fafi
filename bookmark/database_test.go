package bookmark

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// withGlobalBmDb temporarily swaps the package-global BmDb (used by
// CreateOrGet's existence check) and restores it after the test.
func withGlobalBmDb(t *testing.T, r *Database) {
	t.Helper()
	prev := BmDb
	BmDb = r
	t.Cleanup(func() { BmDb = prev })
}

func TestMigrateSchema_FreshDBCreatesLatest(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)

	if err := r.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema: %v", err)
	}
	if r.Version() != schemaVersion {
		t.Errorf("version = %d, want %d", r.Version(), schemaVersion)
	}

	var uv int
	if err := db.QueryRow("PRAGMA user_version").Scan(&uv); err != nil {
		t.Fatal(err)
	}
	if uv != schemaVersion {
		t.Errorf("user_version = %d, want %d", uv, schemaVersion)
	}

	// Verify content_type column exists by inserting a row.
	bm := Bookmark{URL: "https://example.com/", Title: "T", ContentType: "application/pdf"}
	if _, err := r.CreateOrGet(bm); err != nil {
		t.Fatalf("CreateOrGet: %v", err)
	}
	got, err := r.GetByUrl("https://example.com/")
	if err != nil {
		t.Fatalf("GetByUrl: %v", err)
	}
	if got.ContentType != "application/pdf" {
		t.Errorf("ContentType = %q, want application/pdf", got.ContentType)
	}
}

// seedV1 manually constructs the legacy v1 schema (FTS5 without content_type
// + sibling meta table), leaving user_version=0 to mimic pre-versioning installs.
func seedV1(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE VIRTUAL TABLE bookmarks USING FTS5(
		url, title, text, isScraped, dateAdded
	)`); err != nil {
		t.Fatalf("create v1: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE bookmark_meta (
		url TEXT PRIMARY KEY, content_type TEXT
	)`); err != nil {
		t.Fatalf("create bookmark_meta: %v", err)
	}
	now := time.Now().Format(time.RFC3339)
	if _, err := db.Exec(
		`INSERT INTO bookmarks (url, title, text, isScraped, dateAdded) VALUES (?, ?, ?, ?, ?)`,
		"https://example.com/a", "A", "body", 1, now,
	); err != nil {
		t.Fatalf("seed row: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO bookmark_meta (url, content_type) VALUES (?, ?)`,
		"https://example.com/a", "image/png",
	); err != nil {
		t.Fatalf("seed meta: %v", err)
	}
}

func TestMigrateSchema_V1AutoMigratesPreservingContent(t *testing.T) {
	db := openTestDB(t)
	seedV1(t, db)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)

	if err := r.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema: %v", err)
	}
	if r.Version() != schemaVersion {
		t.Fatalf("expected auto-migration to v%d, got %d", schemaVersion, r.Version())
	}

	// bookmark_meta should be gone.
	var name string
	err := db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name='bookmark_meta'",
	).Scan(&name)
	if err != sql.ErrNoRows {
		t.Errorf("bookmark_meta still present: err=%v name=%q", err, name)
	}

	// Row preserved (url, title, dateAdded), text + isScraped cleared.
	got, err := r.GetByUrl("https://example.com/a")
	if err != nil {
		t.Fatalf("GetByUrl: %v", err)
	}
	if got.Title != "A" {
		t.Errorf("Title = %q, want A", got.Title)
	}
	if got.Text != "body" {
		t.Errorf("Text = %q, want preserved 'body' (no reindex)", got.Text)
	}
	if !got.IsScraped.Valid || !got.IsScraped.Bool {
		t.Errorf("IsScraped = %+v, want preserved true", got.IsScraped)
	}
	if got.ContentType != "" {
		t.Errorf("ContentType = %q, want empty (probe runs on next FAFI_RESET_INDEX)", got.ContentType)
	}
}

func TestCreateOrGet_HttpReturnsExistingHttps(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}

	if _, err := r.CreateOrGet(Bookmark{URL: "https://example.com/x", Title: "secure"}); err != nil {
		t.Fatalf("seed https: %v", err)
	}
	got, err := r.CreateOrGet(Bookmark{URL: "http://example.com/x", Title: "insecure"})
	if err != nil {
		t.Fatalf("insert http: %v", err)
	}
	if got.URL != "https://example.com/x" {
		t.Errorf("got URL %q, want https variant", got.URL)
	}
	if got.Title != "secure" {
		t.Errorf("got Title %q, want existing https title 'secure'", got.Title)
	}
}

func TestCreateOrGet_HttpsReplacesExistingHttp(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}

	if _, err := r.CreateOrGet(Bookmark{URL: "http://example.com/y", Title: "old"}); err != nil {
		t.Fatalf("seed http: %v", err)
	}
	if _, err := r.CreateOrGet(Bookmark{URL: "https://example.com/y", Title: "new"}); err != nil {
		t.Fatalf("insert https: %v", err)
	}

	// http row must be gone, https row must be the live one.
	if _, err := r.GetByUrl("http://example.com/y"); err == nil {
		t.Error("http row still exists after https insert")
	}
	got, err := r.GetByUrl("https://example.com/y")
	if err != nil {
		t.Fatalf("GetByUrl https: %v", err)
	}
	if got.Title != "new" {
		t.Errorf("got Title %q, want new", got.Title)
	}
}

func TestMigrateV2toV3_DedupSweep(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)

	// Build a v2 database manually so we can seed http/https dupes before
	// the v2→v3 sweep runs.
	if err := r.createV2(); err != nil {
		t.Fatal(err)
	}
	if err := r.setUserVersion(2); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Format(time.RFC3339)

	// Pair where https has empty scraped data but http has content:
	// content should be moved to the https row before deletion.
	if _, err := db.Exec(
		`INSERT INTO bookmarks (url, title, text, content_type, isScraped, dateAdded)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"http://example.com/a", "A", "scraped body", "text/html", 1, now,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO bookmarks (url, title, text, content_type, dateAdded)
		 VALUES (?, ?, '', '', ?)`,
		"https://example.com/a", "A", now,
	); err != nil {
		t.Fatal(err)
	}

	// Lone http row without an https counterpart — must be preserved.
	if _, err := db.Exec(
		`INSERT INTO bookmarks (url, title, text, content_type, dateAdded)
		 VALUES (?, ?, '', '', ?)`,
		"http://only-http.test/", "lone", now,
	); err != nil {
		t.Fatal(err)
	}

	if err := r.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema: %v", err)
	}
	if r.Version() != schemaVersion {
		t.Fatalf("version = %d, want %d", r.Version(), schemaVersion)
	}

	// http duplicate should be gone, https should have inherited the body.
	if _, err := r.GetByUrl("http://example.com/a"); err == nil {
		t.Error("http duplicate still present after sweep")
	}
	got, err := r.GetByUrl("https://example.com/a")
	if err != nil {
		t.Fatalf("GetByUrl https: %v", err)
	}
	if got.Text != "scraped body" {
		t.Errorf("https Text = %q, want %q (should have inherited from http)", got.Text, "scraped body")
	}
	if got.ContentType != "text/html" {
		t.Errorf("https ContentType = %q, want text/html (inherited)", got.ContentType)
	}

	// Lone http without an https counterpart is untouched.
	if _, err := r.GetByUrl("http://only-http.test/"); err != nil {
		t.Errorf("lone http row was deleted: %v", err)
	}
}

func TestErrBucket_CountsAndFilter(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}
	ok := "https://example.com/ok"
	bad := "https://example.com/dns-fail"
	if _, err := r.CreateOrGet(Bookmark{URL: ok, Title: "ok", Text: "body"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateOrGet(Bookmark{URL: bad, Title: "bad"}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertStatus(ok, 200); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordFailure(bad); err != nil {
		t.Fatal(err)
	}

	counts, err := r.CountStatuses()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Err != 1 {
		t.Errorf("Err count = %d, want 1", counts.Err)
	}
	if counts.None != 0 {
		t.Errorf("None count = %d, want 0 (failed rows go to err, not none)", counts.None)
	}

	got, err := r.AllFiltered("", "err")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].URL != bad {
		t.Errorf("AllFiltered(err) = %v, want [%s]", got, bad)
	}
}

func TestRecordFailure_TracksCounterAndFirstSeen(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}
	url := "https://gone.example/"
	if _, err := r.CreateOrGet(Bookmark{URL: url, Title: "g"}); err != nil {
		t.Fatal(err)
	}

	if err := r.RecordFailure(url); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	n, first, err := r.FailureState(url)
	if err != nil {
		t.Fatalf("FailureState: %v", err)
	}
	if n != 1 {
		t.Errorf("after 1st failure: counter=%d, want 1", n)
	}
	if first.IsZero() {
		t.Errorf("first_failure_at not set after first failure")
	}
	firstStamp := first

	// Second failure increments counter, first_failure_at unchanged.
	if err := r.RecordFailure(url); err != nil {
		t.Fatal(err)
	}
	n, first, _ = r.FailureState(url)
	if n != 2 {
		t.Errorf("after 2nd failure: counter=%d, want 2", n)
	}
	if !first.Equal(firstStamp) {
		t.Errorf("first_failure_at moved: was %v, now %v", firstStamp, first)
	}

	// Success resets the counter.
	if err := r.UpsertStatus(url, 200); err != nil {
		t.Fatal(err)
	}
	n, _, _ = r.FailureState(url)
	if n != 0 {
		t.Errorf("after success: counter=%d, want 0", n)
	}
}

func TestPurgeUnreachable(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}
	// Three seed rows:
	//   keep   — code 0 but has text → keep (manual purge requires empty text)
	//   purge  — code 0 and empty text → soft-delete
	//   alive  — code 200 → keep
	keep := "https://example.com/keep"
	purge := "https://example.com/purge"
	alive := "https://example.com/alive"
	if _, err := r.CreateOrGet(Bookmark{URL: keep, Title: "k", Text: "body"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateOrGet(Bookmark{URL: purge, Title: "p"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateOrGet(Bookmark{URL: alive, Title: "a", Text: "body"}); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordFailure(keep); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordFailure(purge); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertStatus(alive, 200); err != nil {
		t.Fatal(err)
	}

	n, err := r.PurgeUnreachable()
	if err != nil {
		t.Fatalf("PurgeUnreachable: %v", err)
	}
	if n != 1 {
		t.Errorf("purged %d, want 1", n)
	}
	if _, err := r.GetByUrl(purge); err == nil {
		t.Error("purged row still present")
	}
	if !r.IsDeleted(purge) {
		t.Error("purged row not tombstoned")
	}
	if _, err := r.GetByUrl(keep); err != nil {
		t.Errorf("keep row was removed: %v", err)
	}
	if _, err := r.GetByUrl(alive); err != nil {
		t.Errorf("alive row was removed: %v", err)
	}
}

func TestAutoSoftDeleteStaleUnreachable(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}

	// Helper: seed a bookmark and forcibly set its failure state.
	seed := func(url string, fails int, firstAgo time.Duration, text string) {
		if _, err := r.CreateOrGet(Bookmark{URL: url, Title: url, Text: text}); err != nil {
			t.Fatal(err)
		}
		first := time.Now().UTC().Add(-firstAgo).Format(time.RFC3339)
		if _, err := db.Exec(
			`INSERT INTO bookmark_status (url, status_code, last_checked, consecutive_failures, first_failure_at)
			 VALUES (?, 0, ?, ?, ?)
			 ON CONFLICT(url) DO UPDATE SET status_code=0, consecutive_failures=excluded.consecutive_failures, first_failure_at=excluded.first_failure_at`,
			url, time.Now().UTC().Format(time.RFC3339), fails, first,
		); err != nil {
			t.Fatal(err)
		}
	}

	// Eligible: 3 strikes, 15 days old, empty text.
	stale := "https://example.com/stale"
	seed(stale, 3, 15*24*time.Hour, "")

	// Too recent (only 5 days old).
	recent := "https://example.com/recent"
	seed(recent, 5, 5*24*time.Hour, "")

	// Too few strikes (2).
	flaky := "https://example.com/flaky"
	seed(flaky, 2, 30*24*time.Hour, "")

	// Has text — preserved by the rule.
	hasText := "https://example.com/hastext"
	seed(hasText, 5, 30*24*time.Hour, "body")

	n, err := r.AutoSoftDeleteStaleUnreachable()
	if err != nil {
		t.Fatalf("AutoSoftDeleteStaleUnreachable: %v", err)
	}
	if n != 1 {
		t.Errorf("deleted %d, want 1 (only the stale row qualifies)", n)
	}
	if _, err := r.GetByUrl(stale); err == nil {
		t.Error("stale row still present")
	}
	for _, u := range []string{recent, flaky, hasText} {
		if _, err := r.GetByUrl(u); err != nil {
			t.Errorf("%s was removed: %v", u, err)
		}
	}
}

func TestNotes_UpsertGetDelete(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}
	url := "https://example.com/n"
	if _, err := r.CreateOrGet(Bookmark{URL: url, Title: "N"}); err != nil {
		t.Fatal(err)
	}

	// Initially empty.
	got, err := r.GetNote(url)
	if err != nil || got != "" {
		t.Fatalf("expected empty note, got %q err=%v", got, err)
	}

	// Upsert.
	if err := r.UpsertNote(url, "  why bookmarked  "); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}
	got, _ = r.GetNote(url)
	if got != "why bookmarked" {
		t.Errorf("note = %q, want trimmed", got)
	}

	// Empty deletes.
	if err := r.UpsertNote(url, ""); err != nil {
		t.Fatalf("UpsertNote empty: %v", err)
	}
	got, _ = r.GetNote(url)
	if got != "" {
		t.Errorf("note after empty upsert = %q, want empty", got)
	}

	// Cascade on Delete.
	if err := r.UpsertNote(url, "x"); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(url); err != nil {
		t.Fatal(err)
	}
	got, _ = r.GetNote(url)
	if got != "" {
		t.Errorf("note survived bookmark delete: %q", got)
	}
}

// When a status filter is active, rows without scraped text should still be
// returned — a 5xx bookmark typically has no text precisely because the page
// failed to load, but the user wants to see it when filtering by 5xx.
func TestAllFiltered_StatusFilterIncludesEmptyText(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}

	withText := "https://example.com/ok"
	noText := "https://example.com/broken"
	if _, err := r.CreateOrGet(Bookmark{URL: withText, Title: "ok", Text: "body"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateOrGet(Bookmark{URL: noText, Title: "broken"}); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{withText, noText} {
		if err := r.UpsertStatus(u, 503); err != nil {
			t.Fatal(err)
		}
	}

	got, err := r.AllFiltered("", "5xx")
	if err != nil {
		t.Fatalf("AllFiltered: %v", err)
	}
	if len(got) != 2 {
		urls := make([]string, 0, len(got))
		for _, b := range got {
			urls = append(urls, b.URL)
		}
		t.Errorf("got %d rows %v, want 2 (text-less 5xx row must be included)", len(got), urls)
	}
}

// For every status bucket, CountStatuses (used by the chip badge) and
// AllFiltered (the result list) should agree — clicking a chip must show
// exactly that many rows.
func TestCountsMatchFilteredResults_AllBuckets(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}

	// One row with text, one without, for each of 2xx/3xx/4xx/5xx.
	// Plus two "none"-bucket rows (no bookmark_status entry), again
	// one with text, one without.
	cases := []struct {
		bucket string
		code   int
	}{{"2xx", 200}, {"3xx", 301}, {"4xx", 404}, {"5xx", 503}}
	for _, c := range cases {
		withText := "https://example.com/" + c.bucket + "/ok"
		noText := "https://example.com/" + c.bucket + "/notext"
		if _, err := r.CreateOrGet(Bookmark{URL: withText, Title: c.bucket + " ok", Text: "body"}); err != nil {
			t.Fatal(err)
		}
		if _, err := r.CreateOrGet(Bookmark{URL: noText, Title: c.bucket + " notext"}); err != nil {
			t.Fatal(err)
		}
		if err := r.UpsertStatus(withText, c.code); err != nil {
			t.Fatal(err)
		}
		if err := r.UpsertStatus(noText, c.code); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.CreateOrGet(Bookmark{URL: "https://example.com/none/ok", Title: "none ok", Text: "body"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateOrGet(Bookmark{URL: "https://example.com/none/notext", Title: "none notext"}); err != nil {
		t.Fatal(err)
	}

	counts, err := r.CountStatuses()
	if err != nil {
		t.Fatalf("CountStatuses: %v", err)
	}

	checks := []struct {
		bucket string
		want   int
	}{
		{"2xx", counts.S2xx},
		{"3xx", counts.S3xx},
		{"4xx", counts.S4xx},
		{"5xx", counts.S5xx},
		{"none", counts.None},
	}
	for _, c := range checks {
		got, err := r.AllFiltered("", c.bucket)
		if err != nil {
			t.Fatalf("AllFiltered %s: %v", c.bucket, err)
		}
		if len(got) != c.want {
			urls := make([]string, 0, len(got))
			for _, b := range got {
				urls = append(urls, b.URL)
			}
			t.Errorf("bucket %s: chip count=%d, results=%d (%v)", c.bucket, c.want, len(got), urls)
		}
		if c.want != 2 {
			t.Errorf("bucket %s seed sanity: want chip count 2, got %d", c.bucket, c.want)
		}
	}
}

func TestURLsMissingStatus(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}

	hasStatus := "https://example.com/known"
	noStatus := "https://example.com/unknown"
	for _, u := range []string{hasStatus, noStatus} {
		if _, err := r.CreateOrGet(Bookmark{URL: u, Title: "T"}); err != nil {
			t.Fatalf("seed %s: %v", u, err)
		}
	}
	if err := r.UpsertStatus(hasStatus, 200); err != nil {
		t.Fatalf("UpsertStatus: %v", err)
	}

	got, err := r.URLsMissingStatus()
	if err != nil {
		t.Fatalf("URLsMissingStatus: %v", err)
	}
	if len(got) != 1 || got[0] != noStatus {
		t.Errorf("got %v, want exactly [%s]", got, noStatus)
	}

	// After filling it, the list should be empty.
	if err := r.UpsertStatus(noStatus, 404); err != nil {
		t.Fatal(err)
	}
	got, err = r.URLsMissingStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty after fill, got %v", got)
	}

	// A previously-unreachable URL (status=0) should be returned for re-probing
	// so its failure counter can advance.
	if err := r.RecordFailure(hasStatus); err != nil {
		t.Fatal(err)
	}
	got, err = r.URLsMissingStatus()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != hasStatus {
		t.Errorf("expected code-0 row to be re-probed: got %v", got)
	}
}

func TestMigrateSchema_V1WithoutBookmarkMetaTable(t *testing.T) {
	// Real pre-versioning installs don't have bookmark_meta — make sure
	// auto-migration still works.
	db := openTestDB(t)
	if _, err := db.Exec(`CREATE VIRTUAL TABLE bookmarks USING FTS5(
		url, title, text, isScraped, dateAdded
	)`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Format(time.RFC3339)
	if _, err := db.Exec(
		`INSERT INTO bookmarks (url, title, text, isScraped, dateAdded) VALUES (?, ?, ?, ?, ?)`,
		"https://example.com/x", "X", "body", 1, now,
	); err != nil {
		t.Fatal(err)
	}

	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema: %v", err)
	}
	if r.Version() != schemaVersion {
		t.Fatalf("version = %d, want %d", r.Version(), schemaVersion)
	}
	got, err := r.GetByUrl("https://example.com/x")
	if err != nil {
		t.Fatalf("GetByUrl: %v", err)
	}
	if got.Text != "body" {
		t.Errorf("Text = %q, want preserved body", got.Text)
	}
}
