package store

import (
	"time"

	"gorm.io/gorm"
)

const (
	DefaultInterval = 10 * time.Minute
	MinInterval     = 5 * time.Minute
	MaxInterval     = 24 * time.Hour

	PlatformTelegram = "telegram"
)

type Subscription struct {
	gorm.Model

	Platform   string `gorm:"uniqueIndex:idx_dest_url;size:32;not null"`
	ExternalID string `gorm:"uniqueIndex:idx_dest_url;size:64;not null"`
	URL        string `gorm:"uniqueIndex:idx_dest_url;size:2048;not null"`

	IntervalSeconds     int  `gorm:"default:600"`
	Enabled             bool `gorm:"default:true"`
	LastCheckedAt       time.Time
	LastSeenPublished   time.Time
	NextCheckAt         time.Time
}

func ClampInterval(d time.Duration) time.Duration {
	if d < MinInterval {
		return MinInterval
	}
	if d > MaxInterval {
		return MaxInterval
	}
	return d
}
