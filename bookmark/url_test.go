package bookmark

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://vandragt.com", "https://vandragt.com/"},
		{"https://vandragt.com/", "https://vandragt.com/"},
		{"HTTPS://VanDragt.com/foo", "https://vandragt.com/foo"},
		{"https://example.com/page#section", "https://example.com/page"},
		{"https://example.com/path/", "https://example.com/path"},
		{"https://example.com/a/b/", "https://example.com/a/b"},
		{"https://example.com/?utm_source=x&utm_campaign=y", "https://example.com/"},
		{"https://example.com/p?a=1&utm_source=x", "https://example.com/p?a=1"},
		{"https://example.com/?fbclid=abc&q=hello", "https://example.com/?q=hello"},
		{"https://example.com/?utm_source=x", "https://example.com/"},
		{"  https://example.com/  ", "https://example.com/"},
		{"https://example.com:443/", "https://example.com/"},
		{"http://example.com:80/path", "http://example.com/path"},
		{"https://example.com:8443/", "https://example.com:8443/"},
	}
	for _, c := range cases {
		if got := NormalizeURL(c.in); got != c.want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
