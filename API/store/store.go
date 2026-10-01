package store

import (
	"time"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) (*Store, error) {
	if err := db.AutoMigrate(&Subscription{}); err != nil {
		return nil, err
	}
	if err := BackfillDestinations(db); err != nil {
		return nil, err
	}
	if err := ensureDestIndex(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Add(platform, externalID, url string, interval time.Duration) (*Subscription, bool, error) {
	now := time.Now().UTC()
	interval = ClampInterval(interval)
	var sub Subscription
	err := s.db.Where("platform = ? AND external_id = ? AND url = ?", platform, externalID, url).
		First(&sub).Error
	if err == nil {
		sub.Enabled = true
		sub.LastSeenPublished = now
		sub.LastCheckedAt = now
		sub.IntervalSeconds = int(interval.Seconds())
		sub.NextCheckAt = now.Add(interval)
		if serr := s.db.Save(&sub).Error; serr != nil {
			return nil, false, serr
		}
		return &sub, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}
	sub = Subscription{
		Platform:          platform,
		ExternalID:        externalID,
		URL:               url,
		IntervalSeconds:   int(interval.Seconds()),
		Enabled:           true,
		LastCheckedAt:     now,
		LastSeenPublished: now,
		NextCheckAt:       now.Add(interval),
	}
	if cerr := s.db.Create(&sub).Error; cerr != nil {
		return nil, false, cerr
	}
	return &sub, true, nil
}

func (s *Store) Exists(platform, externalID, url string) (bool, error) {
	var n int64
	err := s.db.Model(&Subscription{}).
		Where("platform = ? AND external_id = ? AND url = ?", platform, externalID, url).
		Count(&n).Error
	return n > 0, err
}

func (s *Store) List(platform, externalID string) ([]Subscription, error) {
	var subs []Subscription
	err := s.db.Where("platform = ? AND external_id = ?", platform, externalID).
		Order("id").Find(&subs).Error
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
	res := s.db.Model(&Subscription{}).Where("id = ?", id).Update("enabled", enabled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) Remove(id uint) error {
	res := s.db.Unscoped().Delete(&Subscription{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) SetEnabledByURL(platform, externalID, url string, enabled bool) error {
	res := s.db.Model(&Subscription{}).
		Where("platform = ? AND external_id = ? AND url = ?", platform, externalID, url).
		Update("enabled", enabled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) RemoveByURL(platform, externalID, url string) error {
	res := s.db.Unscoped().
		Where("platform = ? AND external_id = ? AND url = ?", platform, externalID, url).
		Delete(&Subscription{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) SetInterval(platform, externalID string, interval time.Duration, now time.Time) error {
	interval = ClampInterval(interval)
	res := s.db.Model(&Subscription{}).
		Where("platform = ? AND external_id = ?", platform, externalID).
		Updates(map[string]any{
			"interval_seconds": int(interval.Seconds()),
			"next_check_at":    now.UTC().Add(interval),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) Due(now time.Time, limit int) ([]Subscription, error) {
	return s.DueFor("", now, limit)
}

func (s *Store) DueFor(platform string, now time.Time, limit int) ([]Subscription, error) {
	q := s.db.Where("enabled = ? AND next_check_at <= ?", true, now)
	if platform != "" {
		q = q.Where("platform = ?", platform)
	}
	var subs []Subscription
	err := q.Limit(limit).Find(&subs).Error
	return subs, err
}

func (s *Store) AckCursor(id uint, publishedAt, now time.Time) error {
	sub, err := s.Get(id)
	if err != nil {
		return err
	}
	interval := time.Duration(sub.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = DefaultInterval
	}
	return s.db.Model(&Subscription{}).Where("id = ?", id).Updates(map[string]any{
		"last_seen_published": publishedAt,
		"last_checked_at":     now.UTC(),
		"next_check_at":       now.UTC().Add(interval),
	}).Error
}

func (s *Store) MarkChecked(ids []uint, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return s.db.Model(&Subscription{}).Where("id IN ?", ids).
		Updates(map[string]any{"last_checked_at": now}).Error
}

// MarkDue postpones NextCheckAt so a claimed due row is not listed again
// until its interval elapses (same as the old bot MarkChecked-before-enqueue).
func (s *Store) MarkDue(subs []Subscription, now time.Time) error {
	now = now.UTC()
	for i := range subs {
		sub := subs[i]
		interval := time.Duration(sub.IntervalSeconds) * time.Second
		if interval <= 0 {
			interval = DefaultInterval
		}
		if err := s.db.Model(&Subscription{}).Where("id = ?", sub.ID).Updates(map[string]any{
			"last_checked_at": now,
			"next_check_at":   now.Add(interval),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) UpdateCursor(id uint, publishedAt time.Time) error {
	return s.db.Model(&Subscription{}).Where("id = ?", id).
		Update("last_seen_published", publishedAt).Error
}

func (s *Store) FindByURLs(platform, externalID string, urls []string) (*Subscription, error) {
	if len(urls) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var sub Subscription
	err := s.db.Where("platform = ? AND external_id = ? AND url IN ?", platform, externalID, urls).
		First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}
