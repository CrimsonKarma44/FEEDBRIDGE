package utility

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func allowPrivateFetch() bool {
	v := os.Getenv("FEEDBRIDGE_ALLOW_PRIVATE_FETCH")
	return v == "1" || strings.EqualFold(v, "true")
}

func isBlockedHostname(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" || h == "metadata.google.internal" {
		return true
	}
	return strings.HasSuffix(h, ".internal") || strings.HasSuffix(h, ".localhost")
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified()
}

func blockedAddrError(addr string) error {
	return fmt.Errorf("refusing to fetch private or local address %s", addr)
}

func rejectUnsafeURL(u *url.URL) error {
	if allowPrivateFetch() {
		return nil
	}
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("refusing non-http URL")
	}
	host := u.Hostname()
	if isBlockedHostname(host) {
		return blockedAddrError(host)
	}
	if ip := net.ParseIP(host); ip != nil && isBlockedIP(ip) {
		return blockedAddrError(host)
	}
	return nil
}

// NewSafeHTTPClient returns an HTTP client that refuses loopback, private,
// link-local, and metadata destinations at dial time (and on redirects).
// Set FEEDBRIDGE_ALLOW_PRIVATE_FETCH=1 to disable the filter for LAN feeds.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if allowPrivateFetch() {
			return dialer.DialContext(ctx, network, addr)
		}
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if isBlockedHostname(host) {
			return nil, blockedAddrError(host)
		}
		if ip := net.ParseIP(host); ip != nil {
			if isBlockedIP(ip) {
				return nil, blockedAddrError(host)
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		var last error
		for _, ipa := range ips {
			if isBlockedIP(ipa.IP) {
				last = blockedAddrError(ipa.IP.String())
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
			if err == nil {
				return conn, nil
			}
			last = err
		}
		if last == nil {
			last = blockedAddrError(host)
		}
		return nil, last
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return rejectUnsafeURL(req.URL)
		},
	}
}
