package service

import (
	"errors"
	"fmt"

	models "github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
	"gorm.io/gorm"
)

type LinkRepoService struct {
	db *gorm.DB
	models.LinkRepository
}

func NewLinkRepoService(db *gorm.DB) *LinkRepoService {
	return &LinkRepoService{
		db: db,
		LinkRepository: models.LinkRepository{
			FeedLinks: make(map[string]models.FeedType),
		},
	}
}

func (lr *LinkRepoService) GetFeedLink(url string) (models.LinkRepository, error) {
	var link models.LinkRepository
	err := lr.db.Where("url = ?", url).First(&link).Error
	return link, err
}

func (lr *LinkRepoService) AddLink(link models.LinkRepository) error {
	_, err := lr.GetFeedLink(link.Url)

	if err != nil {
		// Check if it's specifically a "not found" error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Record doesn't exist → create a new one
			newLink := models.LinkRepository{
				Url: link.Url,
				FeedLinks: link.FeedLinks,
			}
			return lr.db.Create(&newLink).Error
		}

		// It's a real database error → return it
		return err
	}

	// Record was found → it already exists
	return fmt.Errorf("link already exists: %s", link.Url)
}

func (lr *LinkRepoService) AddFeedLink(repoURL string, feedURL string, feedType models.FeedType) error {
	link, err := lr.GetFeedLink(repoURL)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("repository not found: %s", repoURL)
		}
		return err // real database error
	}

	if link.FeedLinks == nil {
		link.FeedLinks = make(map[string]models.FeedType)
	}

	link.FeedLinks[feedURL] = feedType

	return lr.db.Save(&link).Error
}

func (l *LinkRepoService) GetFeedType(repoURL string, feedURL string) models.FeedType {
	link, err := l.GetFeedLink(repoURL)
	if err != nil {
		return models.FeedTypeUnknown
	}

	if link.FeedLinks == nil {
		return models.FeedTypeUnknown
	}

	if feedType, ok := link.FeedLinks[feedURL]; ok {
		return feedType
	}

	return models.FeedTypeUnknown
}