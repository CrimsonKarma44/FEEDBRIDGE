package handlers

import "testing"

func TestCandidateURLs_InputThenLinksDedupe(t *testing.T) {
	got := candidateURLs("https://example.com", []string{
		"https://example.com/atom.xml",
		"https://example.com",
		"https://example.com/rss.xml",
		"",
	})
	want := []string{
		"https://example.com",
		"https://example.com/atom.xml",
		"https://example.com/rss.xml",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestFirstMatching_PrefersInputThenDetected(t *testing.T) {
	candidates := candidateURLs("https://example.com", []string{
		"https://example.com/atom.xml",
		"https://example.com/rss.xml",
	})
	if got := firstMatching(candidates, []string{"https://example.com/rss.xml"}); got != "https://example.com/rss.xml" {
		t.Fatalf("got %q", got)
	}
	if got := firstMatching(candidates, []string{"https://example.com"}); got != "https://example.com" {
		t.Fatalf("typed URL should win, got %q", got)
	}
	if got := firstMatching(candidates, []string{"https://other.example/feed"}); got != "" {
		t.Fatalf("expected no match, got %q", got)
	}
}

func TestFirstMatching_HomepageResolvesToStoredFeed(t *testing.T) {
	candidates := candidateURLs("https://example.com", []string{
		"https://example.com/atom.xml",
		"https://example.com/rss.xml",
	})
	got := firstMatching(candidates, []string{"https://example.com/atom.xml"})
	if got != "https://example.com/atom.xml" {
		t.Fatalf("got %q, want stored atom URL", got)
	}
}

func TestCandidateURLs_EmptyInputAndNilLinks(t *testing.T) {
	if got := candidateURLs("", nil); len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
	got := candidateURLs("", []string{"https://example.com/feed.xml"})
	if len(got) != 1 || got[0] != "https://example.com/feed.xml" {
		t.Fatalf("got %v", got)
	}
}

func TestFirstMatching_EmptyExisting(t *testing.T) {
	if got := firstMatching([]string{"https://example.com"}, nil); got != "" {
		t.Fatalf("got %q", got)
	}
}
