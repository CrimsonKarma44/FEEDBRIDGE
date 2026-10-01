package store

import (
	"testing"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/internal/testdb"
	"gorm.io/gorm"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db := testdb.Open(t)
	st, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestAdd_IdempotentReenables(t *testing.T) {
	st := newStore(t)
	sub, created, err := st.Add("telegram", "1", "https://example.com/atom.xml", DefaultInterval)
	if err != nil || !created || sub.ID == 0 {
		t.Fatalf("first add: created=%v err=%v id=%d", created, err, sub.ID)
	}
	if err := st.SetEnabled(sub.ID, false); err != nil {
		t.Fatal(err)
	}
	again, created, err := st.Add("telegram", "1", "https://example.com/atom.xml", DefaultInterval)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("second add should not create")
	}
	if again.ID != sub.ID {
		t.Fatalf("id %d != %d", again.ID, sub.ID)
	}
	if !again.Enabled {
		t.Fatal("re-add should re-enable")
	}
}

func TestAdd_SameExternalIDDifferentPlatforms(t *testing.T) {
	st := newStore(t)
	if _, _, err := st.Add("telegram", "1", "https://example.com/feed.xml", DefaultInterval); err != nil {
		t.Fatal(err)
	}
	if _, created, err := st.Add("discord", "1", "https://example.com/feed.xml", DefaultInterval); err != nil || !created {
		t.Fatalf("discord row: created=%v err=%v", created, err)
	}
	tg, err := st.List("telegram", "1")
	if err != nil || len(tg) != 1 {
		t.Fatalf("telegram list=%d err=%v", len(tg), err)
	}
	dc, err := st.List("discord", "1")
	if err != nil || len(dc) != 1 {
		t.Fatalf("discord list=%d err=%v", len(dc), err)
	}
}

func TestDue_RespectsNextCheckAtAndLimit(t *testing.T) {
	st := newStore(t)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	a, _, err := st.Add("telegram", "1", "https://a.example/feed", DefaultInterval)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := st.Add("telegram", "1", "https://b.example/feed", DefaultInterval)
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := st.Add("telegram", "2", "https://c.example/feed", DefaultInterval)
	if err != nil {
		t.Fatal(err)
	}

	if err := st.db.Model(a).Updates(map[string]any{"next_check_at": now.Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := st.db.Model(b).Updates(map[string]any{"next_check_at": now.Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := st.db.Model(c).Updates(map[string]any{"next_check_at": now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}

	due, err := st.Due(now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 2 {
		t.Fatalf("due=%d, want 2", len(due))
	}

	limited, err := st.Due(now, 1)
	if err != nil || len(limited) != 1 {
		t.Fatalf("limit: n=%d err=%v", len(limited), err)
	}
}

func TestSetInterval_ClampsAndBumpsNextCheck(t *testing.T) {
	st := newStore(t)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	_, _, err := st.Add("telegram", "9", "https://example.com/feed", DefaultInterval)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetInterval("telegram", "9", time.Minute, now); err != nil {
		t.Fatal(err)
	}
	subs, err := st.List("telegram", "9")
	if err != nil || len(subs) != 1 {
		t.Fatal(err)
	}
	if subs[0].IntervalSeconds != int(MinInterval.Seconds()) {
		t.Fatalf("interval=%d, want %d", subs[0].IntervalSeconds, int(MinInterval.Seconds()))
	}
	if !subs[0].NextCheckAt.Equal(now.Add(MinInterval)) {
		t.Fatalf("next_check_at=%v", subs[0].NextCheckAt)
	}

	if err := st.SetInterval("telegram", "9", 48*time.Hour, now); err != nil {
		t.Fatal(err)
	}
	subs, _ = st.List("telegram", "9")
	if subs[0].IntervalSeconds != int(MaxInterval.Seconds()) {
		t.Fatalf("max clamp=%d", subs[0].IntervalSeconds)
	}
}

func TestDueFor_FiltersPlatform(t *testing.T) {
	st := newStore(t)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	tg, _, err := st.Add("telegram", "1", "https://tg.example/feed", DefaultInterval)
	if err != nil {
		t.Fatal(err)
	}
	dc, _, err := st.Add("discord", "1", "https://dc.example/feed", DefaultInterval)
	if err != nil {
		t.Fatal(err)
	}
	past := now.Add(-time.Minute)
	if err := st.db.Model(tg).Update("next_check_at", past).Error; err != nil {
		t.Fatal(err)
	}
	if err := st.db.Model(dc).Update("next_check_at", past).Error; err != nil {
		t.Fatal(err)
	}
	due, err := st.DueFor("telegram", now, 10)
	if err != nil || len(due) != 1 || due[0].Platform != "telegram" {
		t.Fatalf("due=%v err=%v", due, err)
	}
}

func TestAckCursor_AdvancesPublishedAndNextCheck(t *testing.T) {
	st := newStore(t)
	sub, _, err := st.Add("telegram", "1", "https://example.com/feed", DefaultInterval)
	if err != nil {
		t.Fatal(err)
	}
	pub := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	if err := st.AckCursor(sub.ID, pub, now); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastSeenPublished.Equal(pub) {
		t.Fatalf("cursor=%v", got.LastSeenPublished)
	}
	if !got.NextCheckAt.Equal(now.Add(DefaultInterval)) {
		t.Fatalf("next=%v", got.NextCheckAt)
	}
}

func TestRemoveByURL_MissingIsNotFound(t *testing.T) {
	st := newStore(t)
	err := st.RemoveByURL("telegram", "1", "https://nope.example/feed")
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("err=%v", err)
	}
}

func TestCandidateURLs_AndFirstMatching(t *testing.T) {
	got := candidateURLs("https://example.com", []string{
		"https://example.com/atom.xml",
		"https://example.com",
		"",
	})
	if len(got) != 2 || got[0] != "https://example.com" || got[1] != "https://example.com/atom.xml" {
		t.Fatalf("got %v", got)
	}
	if firstMatching(got, []string{"https://example.com/atom.xml"}) != "https://example.com/atom.xml" {
		t.Fatal("should match stored atom")
	}
	if firstMatching(got, nil) != "" {
		t.Fatal("empty existing")
	}
}
