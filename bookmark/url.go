package bookmark

import (
	"net/url"
	"strings"
)

// NormalizeURL canonicalises a URL so equivalent forms compare equal.
// Trims whitespace; lowercases scheme and host; strips default ports
// (80 for http, 443 for https); ensures a non-empty path; drops the fragment.
// On parse error the trimmed input is returned unchanged.
func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	switch {
	case u.Scheme == "http" && u.Port() == "80",
		u.Scheme == "https" && u.Port() == "443":
		u.Host = u.Hostname()
	}
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

// otherSchemeURL returns the same URL with http/https swapped, or "" if the
// scheme isn't http/https. Used to find scheme-only duplicates.
func otherSchemeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "https"
	case "https":
		u.Scheme = "http"
	default:
		return ""
	}
	return u.String()
}
