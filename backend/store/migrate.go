package store

import (
	"fmt"

	"gorm.io/gorm"
)

func columnExists(db *gorm.DB, table, column string) bool {
	var n int64
	switch db.Dialector.Name() {
	case "postgres":
		err := db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = ? AND column_name = ?`, table, column).Scan(&n).Error
		return err == nil && n > 0
	default:
		err := db.Raw(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n).Error
		return err == nil && n > 0
	}
}

// BackfillDestinations copies legacy chat_id rows into platform/external_id.
func BackfillDestinations(db *gorm.DB) error {
	if !db.Migrator().HasTable("subscriptions") {
		return nil
	}
	if !columnExists(db, "subscriptions", "chat_id") {
		return nil
	}
	var err error
	if db.Dialector.Name() == "postgres" {
		err = db.Exec(`
			UPDATE subscriptions SET platform = 'telegram' WHERE platform IS NULL OR platform = '';
			UPDATE subscriptions SET external_id = CAST(chat_id AS TEXT)
				WHERE (external_id IS NULL OR external_id = '') AND chat_id IS NOT NULL;
			UPDATE subscriptions SET next_check_at = last_checked_at + (interval_seconds * INTERVAL '1 second')
				WHERE next_check_at IS NULL OR next_check_at = TIMESTAMPTZ '0001-01-01';
		`).Error
	} else {
		err = db.Exec(`UPDATE subscriptions SET platform = 'telegram' WHERE platform IS NULL OR platform = ''`).Error
		if err != nil {
			return fmt.Errorf("backfill platform: %w", err)
		}
		err = db.Exec(`UPDATE subscriptions SET external_id = CAST(chat_id AS TEXT) WHERE (external_id IS NULL OR external_id = '') AND chat_id IS NOT NULL`).Error
		if err != nil {
			return fmt.Errorf("backfill external_id: %w", err)
		}
		err = db.Exec(`UPDATE subscriptions SET next_check_at = datetime(last_checked_at, '+' || interval_seconds || ' seconds') WHERE next_check_at IS NULL OR next_check_at = '' OR next_check_at = '0001-01-01 00:00:00+00:00'`).Error
	}
	if err != nil {
		return fmt.Errorf("backfill destinations: %w", err)
	}
	return nil
}

func ensureDestIndex(db *gorm.DB) error {
	if !db.Migrator().HasTable("subscriptions") {
		return nil
	}
	return db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_dest_url ON subscriptions (platform, external_id, url)`).Error
}
