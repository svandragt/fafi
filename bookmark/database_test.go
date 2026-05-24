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
	if r.Version() != 3 {
		t.Fatalf("version = %d, want 3", r.Version())
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
