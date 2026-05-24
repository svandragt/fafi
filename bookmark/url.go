package bookmark

import (
	"net/url"
	"strings"
)

// trackingParams is the set of query parameters that are stripped during
// normalisation because they only identify the referrer / campaign and never
// change the resource itself.
var trackingParams = map[string]bool{
	"fbclid":  true,
	"gclid":   true,
	"mc_cid":  true,
	"mc_eid":  true,
	"ref":     true,
	"ref_src": true,
	"igshid":  true,
}

// NormalizeURL canonicalises a URL so equivalent forms compare equal.
//   - Trims whitespace.
//   - Lowercases scheme and host.
//   - Strips default ports (80 for http, 443 for https).
//   - Ensures a non-empty path.
//   - Drops fragments.
//   - Drops trailing "/" from non-root paths (so "/a/b/" → "/a/b").
//   - Drops known tracking query parameters (utm_*, fbclid, gclid, …).
//
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
	} else if len(u.Path) > 1 && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	if u.RawQuery != "" {
		u.RawQuery = stripTracking(u.Query())
	}
	return u.String()
}

// stripTracking removes tracking parameters and returns the encoded remainder.
// Returns "" when nothing meaningful is left.
func stripTracking(q url.Values) string {
	for k := range q {
		if trackingParams[k] || strings.HasPrefix(k, "utm_") {
			q.Del(k)
		}
	}
	return q.Encode()
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
