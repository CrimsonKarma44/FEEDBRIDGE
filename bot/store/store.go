package store

import (
	"time"

	"gorm.io/gorm"
)

const (
	DefaultInterval = 10 * time.Minute
	MinInterval     = 5 * time.Minute
	MaxInterval     = 24 * time.Hour
)

type Subscription struct {
	gorm.Model

	ChatID            int64  `gorm:"index:idx_chat_url,unique"`
	URL               string `gorm:"index:idx_chat_url,unique"`
	IntervalSeconds   int    `gorm:"default:600"`
	Enabled           bool   `gorm:"default:true"`
	LastCheckedAt     time.Time
	LastSeenPublished time.Time
}

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) (*Store, error) {
	if err := db.AutoMigrate(&Subscription{}); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Add ensures the chat is subscribed to the URL. Re-adding enables an existing
// subscription. The cursor starts at now so only future items are delivered.
func (s *Store) Add(chatID int64, url string, interval time.Duration) error {
	now := time.Now()
	var sub Subscription
	err := s.db.Where("chat_id = ? AND url = ?", chatID, url).First(&sub).Error
	if err == nil {
		sub.Enabled = true
		sub.LastSeenPublished = now
		return s.db.Save(&sub).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	sub = Subscription{
		ChatID:            chatID,
		URL:               url,
		IntervalSeconds:   int(clampInterval(interval).Seconds()),
		Enabled:           true,
		LastCheckedAt:     now,
		LastSeenPublished: now,
	}
	return s.db.Create(&sub).Error
}

func (s *Store) ListByChat(chatID int64) ([]Subscription, error) {
	var subs []Subscription
	err := s.db.Where("chat_id = ?", chatID).Order("id").Find(&subs).Error
	return subs, err
}

func (s *Store) Get(id uint) (*Subscription, error) {
	var sub Subscription
	if err := s.db.First(&sub, id).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Store) SetEnabled(id uint, enabled bool) error {
	return s.db.Model(&Subscription{}).Where("id = ?", id).Update("enabled", enabled).Error
}

func (s *Store) Remove(id uint) error {
	return s.db.Unscoped().Delete(&Subscription{}, id).Error
}

func (s *Store) SetEnabledByURL(chatID int64, url string, enabled bool) error {
	res := s.db.Model(&Subscription{}).
		Where("chat_id = ? AND url = ?", chatID, url).
		Update("enabled", enabled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) RemoveByURL(chatID int64, url string) error {
	res := s.db.Unscoped().
		Where("chat_id = ? AND url = ?", chatID, url).
		Delete(&Subscription{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) SetChatInterval(chatID int64, interval time.Duration) error {
	return s.db.Model(&Subscription{}).
		Where("chat_id = ?", chatID).
		Update("interval_seconds", int(clampInterval(interval).Seconds())).Error
}

// Due returns enabled subscriptions whose interval has elapsed.
func (s *Store) Due(now time.Time, limit int) ([]Subscription, error) {
	var subs []Subscription
	err := s.db.
		Where("enabled = ? AND EXTRACT(EPOCH FROM (? - last_checked_at)) >= interval_seconds", true, now).
		Limit(limit).
		Find(&subs).Error
	return subs, err
}

func (s *Store) MarkChecked(ids []uint, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.Model(&Subscription{}).Where("id IN ?", ids).Update("last_checked_at", now).Error
}

func (s *Store) UpdateCursor(id uint, publishedAt time.Time) error {
	return s.db.Model(&Subscription{}).Where("id = ?", id).
		Update("last_seen_published", publishedAt).Error
}

func (s *Store) CountByChat(chatID int64) (int64, error) {
	var n int64
	err := s.db.Model(&Subscription{}).Where("chat_id = ?", chatID).Count(&n).Error
	return n, err
}

func clampInterval(d time.Duration) time.Duration {
	if d < MinInterval {
		return MinInterval
	}
	if d > MaxInterval {
		return MaxInterval
	}
	return d
}
