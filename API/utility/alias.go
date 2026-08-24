package utility

import "strings"

func GenerateAlias(url string) string {
	alias := url
	alias = strings.ReplaceAll(alias, "https://", "")
	alias = strings.ReplaceAll(alias, "http://", "")
	alias = strings.ReplaceAll(alias, "/", "")
	alias = strings.ReplaceAll(alias, ".", "")

	return alias
}
