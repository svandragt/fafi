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

func TestMigrateSchema_FreshDBCreatesV2(t *testing.T) {
	db := openTestDB(t)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)

	if err := r.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema: %v", err)
	}
	if r.Version() != 2 {
		t.Errorf("version = %d, want 2", r.Version())
	}

	var uv int
	if err := db.QueryRow("PRAGMA user_version").Scan(&uv); err != nil {
		t.Fatal(err)
	}
	if uv != 2 {
		t.Errorf("user_version = %d, want 2", uv)
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

func TestMigrateSchema_DetectsV1AndResetIndexUpgrades(t *testing.T) {
	db := openTestDB(t)
	seedV1(t, db)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)

	if err := r.MigrateSchema(); err != nil {
		t.Fatalf("MigrateSchema: %v", err)
	}
	if r.Version() != 1 {
		t.Fatalf("expected v1 detected, got %d", r.Version())
	}

	// ResetIndex on v1 should migrate to v2.
	if err := r.ResetIndex(); err != nil {
		t.Fatalf("ResetIndex: %v", err)
	}
	if r.Version() != 2 {
		t.Fatalf("after ResetIndex, version = %d, want 2", r.Version())
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
	if got.Text != "" {
		t.Errorf("Text = %q, want empty (reindex required)", got.Text)
	}
	if got.IsScraped.Valid {
		t.Errorf("IsScraped should be NULL after migration, got %+v", got.IsScraped)
	}
	if got.ContentType != "" {
		t.Errorf("ContentType = %q, want empty after migration (reprobe required)", got.ContentType)
	}
}

func TestUpdate_V1WritesContentTypeToSiblingTable(t *testing.T) {
	db := openTestDB(t)
	seedV1(t, db)
	r := NewDatabase(db)
	withGlobalBmDb(t, r)
	if err := r.MigrateSchema(); err != nil {
		t.Fatal(err)
	}
	if r.Version() != 1 {
		t.Fatalf("expected v1, got %d", r.Version())
	}

	updated := Bookmark{
		Title:       "A",
		Text:        "body",
		ContentType: "application/pdf",
		IsScraped:   sql.NullBool{Bool: true, Valid: true},
	}
	if _, err := r.Update("https://example.com/a", updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var ct string
	if err := db.QueryRow(
		"SELECT content_type FROM bookmark_meta WHERE url = ?",
		"https://example.com/a",
	).Scan(&ct); err != nil {
		t.Fatalf("read meta: %v", err)
	}
	if ct != "application/pdf" {
		t.Errorf("content_type = %q, want application/pdf", ct)
	}
}
