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

	Platform   string `gorm:"size:32"`
	ExternalID string `gorm:"size:64"`
	URL        string `gorm:"size:2048"`

	IntervalSeconds   int  `gorm:"default:600"`
	Enabled           bool `gorm:"default:true"`
	LastCheckedAt     time.Time
	LastSeenPublished time.Time
	NextCheckAt       time.Time
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
