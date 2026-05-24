package bookmark

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

// probeClient is a package-level client so transports and connections are
// pooled across calls. Tests may swap it out.
var probeClient = &http.Client{
	Timeout: 15 * time.Second,
}

// ProbeContentType returns the media type and HTTP status code for the given
// URL. Empty content type means unknown; status 0 means the request never
// reached a response (DNS failure, refused connection, timeout, etc.).
//
// Tries HEAD first; falls back to a ranged GET when HEAD is rejected
// (405/501) or fails, since some servers don't implement HEAD properly.
func ProbeContentType(url string) (string, int, error) {
	ct, status, err := probeHead(url)
	if err == nil && ct != "" {
		return ct, status, nil
	}
	ct2, status2, err2 := probeGet(url)
	if err2 == nil {
		return ct2, status2, nil
	}
	// Prefer a meaningful status from either attempt.
	if status2 == 0 {
		status2 = status
	}
	return ct2, status2, err2
}

func probeHead(url string) (string, int, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", 0, err
	}
	resp, err := probeClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return "", resp.StatusCode, errors.New("HEAD status " + resp.Status)
	}
	return mediaType(resp.Header.Get("Content-Type")), resp.StatusCode, nil
}

func probeGet(url string) (string, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", 0, err
	}
	// Ask for only the first byte to avoid downloading the full body just
	// to read headers. Servers that ignore Range still send headers first.
	req.Header.Set("Range", "bytes=0-0")
	resp, err := probeClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 400 {
		return "", resp.StatusCode, errors.New("GET status " + resp.Status)
	}
	return mediaType(resp.Header.Get("Content-Type")), resp.StatusCode, nil
}

// mediaType strips parameters from a Content-Type header value
// ("text/html; charset=utf-8" → "text/html") and lowercases.
func mediaType(header string) string {
	if header == "" {
		return ""
	}
	mt, _, err := mime.ParseMediaType(header)
	if err != nil {
		// Fall back to the part before the first ';' if any.
		if i := strings.IndexByte(header, ';'); i >= 0 {
			return strings.ToLower(strings.TrimSpace(header[:i]))
		}
		return strings.ToLower(strings.TrimSpace(header))
	}
	return mt
}

// IsTextual reports whether a media type should go through readability/article
// extraction. Anything text/* qualifies; empty (unknown) defaults to true so
// extraction is attempted (preserving legacy behaviour).
func IsTextual(contentType string) bool {
	if contentType == "" {
		return true
	}
	return strings.HasPrefix(contentType, "text/")
}
