package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func boolPtr(v bool) *bool { return &v }

func TestBackupRoundTrip(t *testing.T) {
	when := time.Date(2026, 10, 17, 15, 49, 9, 0, time.UTC)
	in := []knownFeed{
		{URL: "https://example.com/feed.xml", Enabled: true, IntervalSeconds: 600},
		{URL: "https://other.example/atom.xml", Enabled: false, IntervalSeconds: 1800},
		{URL: "https://third.example/rss", Enabled: true, IntervalSeconds: 600},
	}
	file := buildBackup(in, when)
	raw, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseBackup(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 {
		t.Fatalf("version %d", got.Version)
	}
	if !got.ExportedAt.Equal(when) {
		t.Fatalf("exported_at %s", got.ExportedAt)
	}
	if got.IntervalSeconds == nil || *got.IntervalSeconds != 600 {
		t.Fatalf("interval %+v, want 600 (most common)", got.IntervalSeconds)
	}
	if len(got.Feeds) != len(in) {
		t.Fatalf("feeds %d", len(got.Feeds))
	}
	for i, feed := range got.Feeds {
		if feed.URL != in[i].URL || feed.Enabled == nil || *feed.Enabled != in[i].Enabled {
			t.Fatalf("feed %d: %+v", i, feed)
		}
	}
}

func TestBackupIntervalTiePrefersSmaller(t *testing.T) {
	file := buildBackup([]knownFeed{
		{URL: "https://example.com/a", Enabled: true, IntervalSeconds: 1800},
		{URL: "https://example.com/b", Enabled: true, IntervalSeconds: 600},
	}, time.Unix(0, 0).UTC())
	if file.IntervalSeconds == nil || *file.IntervalSeconds != 600 {
		t.Fatalf("interval %+v", file.IntervalSeconds)
	}
}

func TestBackupEmptyOmitsInterval(t *testing.T) {
	file := buildBackup(nil, time.Unix(0, 0).UTC())
	raw, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "interval_seconds") {
		t.Fatalf("empty backup should omit interval: %s", raw)
	}
	got, err := parseBackup(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Feeds) != 0 || got.IntervalSeconds != nil {
		t.Fatalf("%+v", got)
	}
}

func TestParseBackupRejects(t *testing.T) {
	feeds := make([]backupFeed, maxBackupFeeds+1)
	enabled := true
	for i := range feeds {
		feeds[i] = backupFeed{URL: "https://example.com/feed", Enabled: &enabled}
	}
	tooMany, err := json.Marshal(backupFile{Version: 1, Feeds: feeds})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		raw  string
	}{
		{"bad version", `{"version":2,"feeds":[]}`},
		{"missing version", `{"feeds":[]}`},
		{"not json", `not-json`},
		{"empty", `   `},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseBackup([]byte(tc.raw)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	t.Run("too many feeds", func(t *testing.T) {
		_, err := parseBackup(tooMany)
		if err == nil || !strings.Contains(err.Error(), "maximum") {
			t.Fatalf("err %v", err)
		}
	})
}

func TestPlanRestore(t *testing.T) {
	existing := []knownFeed{
		{URL: "https://example.com/feed.xml", Enabled: true, IntervalSeconds: 600},
		{URL: "https://paused.example/rss", Enabled: false, IntervalSeconds: 600},
	}
	file := backupFile{
		Version: 1,
		Feeds: []backupFeed{
			{URL: "example.com/feed.xml", Enabled: boolPtr(true)},
			{URL: "https://paused.example/rss/", Enabled: boolPtr(true)},
			{URL: "https://new.example/atom", Enabled: boolPtr(false)},
			{URL: "https://new.example/atom", Enabled: boolPtr(true)},
			{URL: "ftp://example.com/x", Enabled: boolPtr(true)},
			{URL: "https://missing-flag.example/rss", Enabled: nil},
		},
		IntervalSeconds: intPtr(1),
	}
	plan := planRestore(existing, file)
	if len(plan.Feeds) != 5 {
		t.Fatalf("actions %d, want 5 (duplicate skipped): %+v", len(plan.Feeds), plan.Feeds)
	}
	want := []actionKind{actionUnchanged, actionEnable, actionSubscribe, actionInvalid, actionInvalid}
	for i, kind := range want {
		if plan.Feeds[i].Kind != kind {
			t.Fatalf("action %d kind %d, want %d (%+v)", i, plan.Feeds[i].Kind, kind, plan.Feeds[i])
		}
	}
	if plan.Feeds[0].StoredURL != existing[0].URL {
		t.Fatalf("stored url %q", plan.Feeds[0].StoredURL)
	}
	if plan.Feeds[1].Kind != actionEnable || plan.Feeds[1].StoredURL != existing[1].URL {
		t.Fatalf("enable action %+v", plan.Feeds[1])
	}
	if plan.Feeds[2].Kind != actionSubscribe || plan.Feeds[2].Enabled || plan.Feeds[2].URL != "https://new.example/atom" {
		t.Fatalf("subscribe action %+v", plan.Feeds[2])
	}
	if !plan.SetInterval || plan.Interval != minInterval || !plan.IntervalClamped {
		t.Fatalf("interval %+v", plan)
	}
}

func TestPlanRestoreSameFlagDoesNothing(t *testing.T) {
	existing := []knownFeed{{URL: "https://example.com/feed.xml", Enabled: false}}
	plan := planRestore(existing, backupFile{
		Version: 1,
		Feeds:   []backupFeed{{URL: "https://example.com/feed.xml", Enabled: boolPtr(false)}},
	})
	if len(plan.Feeds) != 1 || plan.Feeds[0].Kind != actionUnchanged || plan.SetInterval {
		t.Fatalf("%+v", plan)
	}
}

func TestPlanRestoreDisableExisting(t *testing.T) {
	existing := []knownFeed{{URL: "https://example.com/feed.xml", Enabled: true}}
	plan := planRestore(existing, backupFile{
		Version: 1,
		Feeds:   []backupFeed{{URL: "https://example.com/feed.xml", Enabled: boolPtr(false)}},
	})
	if len(plan.Feeds) != 1 || plan.Feeds[0].Kind != actionDisable {
		t.Fatalf("%+v", plan.Feeds)
	}
}

func TestIsRestoreCaption(t *testing.T) {
	ok := []string{"/restore", "/restore@feed_bridge_bot", "  /Restore extra", "/restore@Feed_Bridge_Bot"}
	for _, caption := range ok {
		if !isRestoreCaption(caption, "feed_bridge_bot") {
			t.Fatalf("want restore: %q", caption)
		}
	}
	no := []string{"", "restore", "/backup", "please /restore", "/addfeed", "/restore@other_bot"}
	for _, caption := range no {
		if isRestoreCaption(caption, "feed_bridge_bot") {
			t.Fatalf("want ignore: %q", caption)
		}
	}
}

func TestRestoreSlotsExpire(t *testing.T) {
	var slots restoreSlots
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	if slots.state(7, now) != waitNone {
		t.Fatal("empty slot")
	}
	slots.arm(7, now)
	if slots.state(7, now.Add(9*time.Minute)) != waitReady {
		t.Fatal("expected ready")
	}
	if slots.state(7, now.Add(restoreWait)) != waitExpired {
		t.Fatal("expected expired")
	}
	if slots.state(7, now.Add(restoreWait+time.Second)) != waitNone {
		t.Fatal("expired slot should be cleared")
	}
}

func TestFailureLineEscapesHTML(t *testing.T) {
	line := failureLine("https://example.com?a=1&b=2", "no <feeds>")
	if !strings.Contains(line, "&amp;") || !strings.Contains(line, "&lt;feeds&gt;") {
		t.Fatalf("%s", line)
	}
}

func TestFormatRestoreSummary(t *testing.T) {
	empty := formatRestoreSummary(restoreResult{empty: true})
	if empty != "Nothing to restore. The file has no feeds." {
		t.Fatalf("%s", empty)
	}
	body := formatRestoreSummary(restoreResult{
		added:    1,
		failures: []string{"https://bad.example — no feeds were found"},
	})
	if !strings.Contains(body, "Added: 1.") || !strings.Contains(body, "Failed: 1.") {
		t.Fatalf("%s", body)
	}
	many := make([]string, maxFailureLines+2)
	for i := range many {
		many[i] = "x"
	}
	trimmed := formatRestoreSummary(restoreResult{failures: many})
	if !strings.Contains(trimmed, "and 2 more") {
		t.Fatalf("%s", trimmed)
	}
}

func intPtr(v int) *int { return &v }
