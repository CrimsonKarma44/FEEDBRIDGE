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
		{"port kept", "localhost:50051/feed", "https://localhost:50051/feed", false},
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
