package bookmark

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeContentType_HEAD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ct, err := ProbeContentType(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ct != "application/pdf" {
		t.Errorf("got %q, want application/pdf", ct)
	}
}

func TestProbeContentType_StripsParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}))
	defer srv.Close()

	ct, err := ProbeContentType(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ct != "text/html" {
		t.Errorf("got %q, want text/html", ct)
	}
}

func TestProbeContentType_HEADRejectedFallsBackToGET(t *testing.T) {
	var gotGET bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Method == http.MethodGet {
			gotGET = true
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte{0})
		}
	}))
	defer srv.Close()

	ct, err := ProbeContentType(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotGET {
		t.Error("expected GET fallback to be invoked")
	}
	if ct != "image/png" {
		t.Errorf("got %q, want image/png", ct)
	}
}

func TestIsTextual(t *testing.T) {
	cases := map[string]bool{
		"":                true,
		"text/html":       true,
		"text/plain":      true,
		"application/pdf": false,
		"image/png":       false,
	}
	for in, want := range cases {
		if got := IsTextual(in); got != want {
			t.Errorf("IsTextual(%q) = %v, want %v", in, got, want)
		}
	}
}
