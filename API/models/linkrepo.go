package models

import "gorm.io/gorm"

type LinkRepository struct {
	gorm.Model

	Url       string              `gorm:"uniqueIndex"`
	FeedLinks map[string]FeedType `gorm:"serializer:json;type:jsonb"`
}

type FeedType string

const (
	FeedTypeRSS     FeedType = "rss"
	FeedTypeAtom    FeedType = "atom"
	FeedTypeJSON    FeedType = "json"
	FeedTypeUnknown FeedType = "unknown"
)
