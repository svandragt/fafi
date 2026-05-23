package bookmark

import (
	"net/url"
	"strings"
)

// NormalizeURL canonicalises a URL so equivalent forms compare equal.
// Lowercases scheme and host, ensures a non-empty path, and drops the fragment.
// On parse error the input is returned unchanged.
func NormalizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}
