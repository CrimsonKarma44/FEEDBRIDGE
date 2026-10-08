package handlers

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

const maxURLLen = 2048

var schemeRe = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9+.-]*)://`)
var hostRe = regexp.MustCompile(`^[a-zA-Z0-9.\-]+$`)

// NormalizeURL cleans and validates a user-supplied feed URL.
// Scheme-less URLs get https:// prepended; trailing slashes are trimmed so
// later lookups match regardless of how the user typed it.
func NormalizeURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("it is empty")
	}
	if strings.ContainsAny(trimmed, " \t\n\r") {
		return "", fmt.Errorf("it contains spaces")
	}
	if len(trimmed) > maxURLLen {
		return "", fmt.Errorf("it is too long")
	}

	switch m := schemeRe.FindStringSubmatch(trimmed); {
	case m == nil:
		trimmed = "https://" + trimmed
	case m[1] == "http" || m[1] == "https":
	default:
		return "", fmt.Errorf("only http and https are supported")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("it could not be parsed")
	}

	host := u.Hostname()
	if host == "" || !hostRe.MatchString(host) {
		return "", fmt.Errorf("it is missing a valid host")
	}
	if isLocalOrPrivateHost(host) {
		return "", fmt.Errorf("it points at a private or local address")
	}

	return strings.TrimRight(trimmed, "/"), nil
}

func isLocalOrPrivateHost(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" || h == "metadata.google.internal" {
		return true
	}
	if strings.HasSuffix(h, ".internal") || strings.HasSuffix(h, ".localhost") {
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// BadURLHelp renders friendly feedback for a rejected URL.
func BadURLHelp(err error) string {
	return "That doesn't look like a valid feed address: " + err.Error() + ".\n" +
		"Example: /addfeed https://example.com"
}
