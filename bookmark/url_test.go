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
		{"https://example.com/path/", "https://example.com/path/"},
	}
	for _, c := range cases {
		if got := NormalizeURL(c.in); got != c.want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
