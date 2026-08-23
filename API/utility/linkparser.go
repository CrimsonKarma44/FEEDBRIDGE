package utility

import (
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
)

func ToFeedTypeMap(m map[string]string) map[string]models.FeedType {
	result := make(map[string]models.FeedType, len(m))
	for k, v := range m {
		result[k] = models.FeedType(v) // cast string → FeedType
	}
	return result
}

func ToStringMap(m map[string]models.FeedType) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = string(v) // cast FeedType → string
	}
	return result
}
