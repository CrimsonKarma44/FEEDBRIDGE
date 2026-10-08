package handlers

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"empty", "", "", true},
		{"whitespace only", "   \t ", "", true},
		{"internal space", "https://example.com/a b", "", true},
		{"plain host", "example.com", "https://example.com", false},
		{"host with path", "example.com/blog", "https://example.com/blog", false},
		{"trailing slash", "https://example.com/blog/", "https://example.com/blog", false},
		{"bare domain trailing slash", "example.com/", "https://example.com", false},
		{"keeps http", "http://example.com/feed", "http://example.com/feed", false},
		{"keeps query params", "https://example.com/feed?utm=x&y=1", "https://example.com/feed?utm=x&y=1", false},
		{"rejects ftp", "ftp://example.com/file", "", true},
		{"rejects javascript", "javascript:alert(1)", "", true},
		{"missing host", "https://", "", true},
		{"unparseable control char", "https://exa\nmple.com", "", true},
		{"rejects localhost", "localhost:50051/feed", "", true},
		{"rejects LOCALHOST", "http://LOCALHOST/feed", "", true},
		{"rejects foo.localhost", "http://foo.localhost/feed", "", true},
		{"rejects loopback", "http://127.0.0.1/feed", "", true},
		{"rejects loopback no scheme", "127.0.0.1/feed", "", true},
		{"rejects unspecified", "http://0.0.0.0/", "", true},
		{"rejects rfc1918 10", "http://10.0.0.5/rss", "", true},
		{"rejects rfc1918 192.168", "http://192.168.1.9/rss", "", true},
		{"rejects rfc1918 172.16", "http://172.16.0.1/rss", "", true},
		{"rejects rfc1918 172.31", "http://172.31.255.255/rss", "", true},
		{"allows 172.15 (not rfc1918)", "http://172.15.0.1/rss", "http://172.15.0.1/rss", false},
		{"rejects metadata ip", "http://169.254.169.254/", "", true},
		{"rejects metadata host", "http://metadata.google.internal/", "", true},
		{"rejects .internal", "http://redis.internal/feed", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeURL(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeURL(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeURL(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
