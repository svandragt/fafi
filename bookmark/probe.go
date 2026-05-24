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

// ProbeContentType returns the media type (e.g. "text/html", "application/pdf")
// for the given URL, with no parameters. Empty string means unknown.
//
// Tries HEAD first; falls back to a ranged GET when HEAD is rejected
// (405/501) or fails, since some servers don't implement HEAD properly.
func ProbeContentType(url string) (string, error) {
	if ct, err := probeHead(url); err == nil && ct != "" {
		return ct, nil
	}
	return probeGet(url)
}

func probeHead(url string) (string, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := probeClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return "", errors.New("HEAD status " + resp.Status)
	}
	return mediaType(resp.Header.Get("Content-Type")), nil
}

func probeGet(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	// Ask for only the first byte to avoid downloading the full body just
	// to read headers. Servers that ignore Range still send headers first.
	req.Header.Set("Range", "bytes=0-0")
	resp, err := probeClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 400 {
		return "", errors.New("GET status " + resp.Status)
	}
	return mediaType(resp.Header.Get("Content-Type")), nil
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
