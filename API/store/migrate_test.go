package store

import (
	"testing"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/internal/testdb"
)

func TestBackfillDestinations_FromLegacyChatID(t *testing.T) {
	db := testdb.Open(t)
	if err := db.Exec(`CREATE TABLE subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME,
		chat_id INTEGER,
		url TEXT,
		interval_seconds INTEGER DEFAULT 600,
		enabled INTEGER DEFAULT 1,
		last_checked_at DATETIME,
		last_seen_published DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	checked := time.Date(2026, 10, 1, 11, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO subscriptions (id, chat_id, url, interval_seconds, enabled, last_checked_at, last_seen_published)
		VALUES (1, 42, 'https://example.com/feed', 600, 1, ?, ?)`, checked, checked).Error; err != nil {
		t.Fatal(err)
	}

	st, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Platform != PlatformTelegram || got.ExternalID != "42" {
		t.Fatalf("platform=%q external_id=%q", got.Platform, got.ExternalID)
	}
	if got.URL != "https://example.com/feed" {
		t.Fatalf("url=%q", got.URL)
	}
	if got.NextCheckAt.IsZero() {
		t.Fatal("next_check_at not backfilled")
	}
}
