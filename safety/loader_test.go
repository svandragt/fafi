package safety

import (
	"os"
	"testing"
)

func TestParseURLhaus_SkipsCommentsAndOffline(t *testing.T) {
	f, err := os.Open("testdata/urlhaus_sample.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	set, n, err := parseURLhaus(f)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 online entries, got %d", n)
	}
	if _, ok := set.Check("https://evil.example/a.exe"); !ok {
		t.Error("expected exact-match hit for evil.example/a.exe")
	}
	if _, ok := set.Check("http://bad.example:8080/payload"); !ok {
		t.Error("expected exact-match hit for bad.example payload")
	}
	if _, ok := set.Check("https://taken-down.example/x"); ok {
		t.Error("offline entries must not be loaded")
	}
	// Host fallback: a different path on a listed host should still hit.
	if _, ok := set.Check("https://evil.example/some-other-page"); !ok {
		t.Error("expected host fallback on evil.example")
	}
}

func TestParseURLhaus_RespectsSizeCap(t *testing.T) {
	f, err := os.Open("testdata/urlhaus_sample.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	prev := maxFeedBytes
	maxFeedBytes = 64 // tiny — forces truncation in the middle of the header
	defer func() { maxFeedBytes = prev }()

	if _, _, err := parseURLhaus(f); err == nil {
		t.Fatal("expected size-cap error")
	}
}
