package models

import (
	"time"

	"github.com/mmcdole/gofeed"
	"gorm.io/gorm"
)

type FeedItem struct {
	Title       string
	Description string

	Links   []string
	Authors []*person `json:"authors,omitempty"`

	Author string

	Image      *image       `json:"image,omitempty"`
	Enclosures []*enclosure `json:"enclosures,omitempty"`

	PublishedAt *time.Time
}

type person struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

type image struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

type enclosure struct {
	URL    string `json:"url"`
	Type   string `json:"type"`
	Length string `json:"length"`
}

func NewFeedItem(feed *gofeed.Item) *FeedItem {
	authors := make([]*person, 0, len(feed.Authors))
	for _, author := range feed.Authors {
		authors = append(authors, &person{Name: author.Name, Email: author.Email})
	}
	enclosures := make([]*enclosure, 0, len(feed.Enclosures))
	for _, enclose := range feed.Enclosures {
		enclosures = append(enclosures, &enclosure{URL: enclose.URL, Type: enclose.Type, Length: enclose.Length})
	}
	if feed.Image == nil {
		feed.Image = &gofeed.Image{}
	}
	image := &image{URL: feed.Image.URL, Title: feed.Image.Title}
	return &FeedItem{
		Title:       feed.Title,
		Description: feed.Description,
		Links:       feed.Links,
		Authors:     authors,
		Image:       image,
		Enclosures:  enclosures,
		PublishedAt: feed.PublishedParsed,
	}
}

type LinkRepository struct {
	gorm.Model

	url       string `gorm:"uniqueIndex"`
	feedLinks map[string]FeedType
}

type FeedType string

const (
	FeedTypeRSS     FeedType = "rss"
	FeedTypeAtom    FeedType = "atom"
	FeedTypeJSON    FeedType = "json"
	FeedTypeUnknown FeedType = "unknown"
)
