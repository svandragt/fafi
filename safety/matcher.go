// Package safety provides a local, privacy-preserving URL blocklist check.
//
// Lookups happen entirely in-memory against a snapshot loaded from a public
// feed (URLhaus). No per-URL network traffic, so the user's bookmarks never
// leave the machine — only the periodic feed download does.
package safety

import (
	"net/url"
	"strings"
)

// Verdict is the source-qualified label written into the bookmark_safety
// sibling table. The "urlhaus" verdict means the URL (or its host) appeared
// in the URLhaus online feed at last refresh.
const Verdict = "urlhaus"

// Set is an in-memory blocklist supporting exact-URL and host-fallback
// lookups. Not safe for concurrent mutation, but concurrent reads are fine
// once populated; callers swap whole Set values atomically via Loader.
type Set struct {
	urls  map[string]struct{}
	hosts map[string]struct{}
}

func NewSet() *Set {
	return &Set{
		urls:  make(map[string]struct{}),
		hosts: make(map[string]struct{}),
	}
}

// Len returns the number of distinct (normalized) URLs in the set. Used for
// telemetry / log lines.
func (s *Set) Len() int {
	if s == nil {
		return 0
	}
	return len(s.urls)
}

// AddURL inserts one feed entry, populating both the URL and host indexes.
// Malformed inputs are silently ignored — the feed is a best-effort source.
func (s *Set) AddURL(raw string) {
	n, host := normalize(raw)
	if n == "" {
		return
	}
	s.urls[n] = struct{}{}
	if host != "" {
		s.hosts[host] = struct{}{}
	}
}

// Check returns the verdict label and whether the URL is blocked. It first
// tries an exact (normalized) match, then falls back to the host. The
// host-fallback rule is deliberately broad: if any URL on a host was reported
// malicious, treat the host as suspect.
func (s *Set) Check(raw string) (string, bool) {
	if s == nil {
		return "", false
	}
	n, host := normalize(raw)
	if n == "" {
		return "", false
	}
	if _, ok := s.urls[n]; ok {
		return Verdict, true
	}
	if host != "" {
		if _, ok := s.hosts[host]; ok {
			return Verdict, true
		}
	}
	return "", false
}

// normalize returns (canonical-url, host) suitable for map lookup, or
// ("", "") when the input doesn't parse as an absolute URL with a host.
func normalize(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", ""
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	// Strip a bare trailing slash on the path so "https://x/" and "https://x"
	// hash the same. Preserve "/" itself for paths that are only the slash —
	// otherwise we'd canonicalise it to an empty path, which url.String()
	// then re-emits without one anyway, so this is safe.
	if u.Path == "/" {
		u.Path = ""
	} else {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	u.Fragment = ""
	return u.String(), u.Host
}
