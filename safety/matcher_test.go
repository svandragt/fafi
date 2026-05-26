package safety

import "testing"

func TestSet_ExactMatch(t *testing.T) {
	s := NewSet()
	s.AddURL("https://evil.example/page.php?id=1")

	if v, ok := s.Check("https://evil.example/page.php?id=1"); !ok || v != "urlhaus" {
		t.Fatalf("expected exact hit, got verdict=%q ok=%v", v, ok)
	}
}

func TestSet_NormalizeTrailingSlash(t *testing.T) {
	s := NewSet()
	s.AddURL("https://evil.example/")

	if _, ok := s.Check("https://evil.example"); !ok {
		t.Fatal("expected match after trailing-slash normalization")
	}
}

func TestSet_HostFallback(t *testing.T) {
	s := NewSet()
	s.AddURL("https://evil.example/a") // adds host evil.example

	if _, ok := s.Check("https://evil.example/totally-different-path"); !ok {
		t.Fatal("expected host fallback to match")
	}
}

func TestSet_Miss(t *testing.T) {
	s := NewSet()
	s.AddURL("https://evil.example/a")

	if _, ok := s.Check("https://safe.example/a"); ok {
		t.Fatal("expected miss for unrelated host")
	}
}

func TestSet_HostCaseInsensitive(t *testing.T) {
	s := NewSet()
	s.AddURL("https://Evil.Example/a")

	if _, ok := s.Check("https://EVIL.example/b"); !ok {
		t.Fatal("expected case-insensitive host match")
	}
}

func TestSet_IgnoresInvalidURL(t *testing.T) {
	s := NewSet()
	s.AddURL("not a url")
	// Should not panic and should not match anything.
	if _, ok := s.Check("https://anything.example/"); ok {
		t.Fatal("invalid input should not produce matches")
	}
}
