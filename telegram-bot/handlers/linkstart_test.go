package handlers

import (
	"strings"
	"testing"
)

func TestLinkToken(t *testing.T) {
	cases := []struct {
		payload    string
		wantToken  string
		wantIsLink bool
	}{
		{"add_130ps112PTd1", "130ps112PTd1", true},
		{"  add_f7pL69LGbq-g  ", "f7pL69LGbq-g", true},
		{"add_short", "", true},         // looks like a link, malformed token
		{"add_130ps112PTd1x", "", true}, // too long
		{"add_130ps112PT$1", "", true},  // bad character
		{"add_", "", true},
		{"", "", false},                 // plain /start
		{"hello", "", false},            // some other payload
		{"ADD_130ps112PTd1", "", false}, // prefix is case-sensitive
	}
	for _, tc := range cases {
		tok, isLink := linkToken(tc.payload)
		if tok != tc.wantToken || isLink != tc.wantIsLink {
			t.Errorf("linkToken(%q) = (%q, %v), want (%q, %v)", tc.payload, tok, isLink, tc.wantToken, tc.wantIsLink)
		}
	}
}

func TestLinkPayloadFitsTelegram(t *testing.T) {
	// Telegram caps start payloads at 64 chars and callback data at 64 bytes.
	tok := "130ps112PTd1"
	if n := len(linkPayloadPrefix + tok); n > 64 {
		t.Fatalf("start payload is %d chars", n)
	}
	for _, unique := range []string{"lk_add", "lk_no"} {
		if n := len("\f" + unique + "|" + tok); n > 64 {
			t.Fatalf("callback data for %s is %d bytes", unique, n)
		}
	}
}

func TestLinkConfirmText(t *testing.T) {
	private := linkConfirmText("https://example.com/feed?a=1&b=<2>", false)
	if !strings.Contains(private, "https://example.com/feed?a=1&amp;b=&lt;2&gt;") {
		t.Fatalf("URL not escaped: %s", private)
	}
	if strings.Contains(private, "admins") {
		t.Fatalf("private chat text mentions admins: %s", private)
	}
	if group := linkConfirmText("https://example.com/feed", true); !strings.Contains(group, "Only group admins can confirm") {
		t.Fatalf("group text missing admin note: %s", group)
	}
}
