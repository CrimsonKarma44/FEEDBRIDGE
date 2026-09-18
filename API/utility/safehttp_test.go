package utility

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRejectUnsafeURL(t *testing.T) {
	cases := []struct {
		raw     string
		blocked bool
	}{
		{"https://example.com/feed", false},
		{"http://127.0.0.1/feed", true},
		{"http://[::1]/feed", true},
		{"http://10.0.0.1/feed", true},
		{"http://192.168.1.1/feed", true},
		{"http://172.16.0.1/feed", true},
		{"http://172.31.255.255/feed", true},
		{"http://172.15.0.1/feed", false},
		{"http://169.254.169.254/latest/meta-data", true},
		{"http://localhost/feed", true},
		{"http://LOCALHOST/feed", true},
		{"http://foo.localhost/feed", true},
		{"http://metadata.google.internal/", true},
		{"http://foo.internal/feed", true},
		{"ftp://example.com/feed", true},
		{"https://8.8.8.8/feed", false},
	}
	for _, tc := range cases {
		u, err := url.Parse(tc.raw)
		if err != nil {
			t.Fatalf("parse %s: %v", tc.raw, err)
		}
		err = rejectUnsafeURL(u)
		if tc.blocked && err == nil {
			t.Errorf("%s: want blocked", tc.raw)
		}
		if !tc.blocked && err != nil {
			t.Errorf("%s: unexpected %v", tc.raw, err)
		}
	}
}

func TestSafeHTTPClient_BlocksLoopback(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	client := NewSafeHTTPClient(2 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+ln.Addr().String()+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(req)
	if err == nil {
		t.Fatal("expected loopback to be blocked")
	}
}

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", "10.1.2.3", "192.168.0.9", "172.16.0.1", "172.31.255.255",
		"169.254.169.254", "0.0.0.0", "fe80::1", "fc00::1",
	}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "172.15.0.1", "172.32.0.1", "2001:4860:4860::8888"}
	for _, s := range allowed {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
	if !isBlockedIP(nil) {
		t.Error("nil IP should be blocked")
	}
}

func TestSafeHTTPClient_BlocksPrivateLiteralWithoutDial(t *testing.T) {
	client := NewSafeHTTPClient(2 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://169.254.169.254/", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Do(req)
	if err == nil {
		t.Fatal("expected metadata IP to be blocked")
	}
	if !strings.Contains(err.Error(), "refusing to fetch") {
		t.Fatalf("err = %v, want refusing to fetch", err)
	}
}

func TestSafeHTTPClient_CheckRedirectBlocksPrivate(t *testing.T) {
	client := NewSafeHTTPClient(2 * time.Second)
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/latest", nil)
	if err != nil {
		t.Fatal(err)
	}
	via := []*http.Request{req}
	if err := client.CheckRedirect(req, via); err == nil {
		t.Fatal("redirect to loopback should be blocked")
	}
	okReq, err := http.NewRequest(http.MethodGet, "https://example.com/feed", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(okReq, via); err != nil {
		t.Fatalf("public redirect: %v", err)
	}
}

func TestAllowPrivateFetch_BypassesReject(t *testing.T) {
	t.Setenv("FEEDBRIDGE_ALLOW_PRIVATE_FETCH", "1")
	u, _ := url.Parse("http://127.0.0.1/feed")
	if err := rejectUnsafeURL(u); err != nil {
		t.Fatalf("allow-private should accept loopback, got %v", err)
	}
}

func TestIsBlockedHostname(t *testing.T) {
	blocked := []string{"localhost", "LOCALHOST", "localhost.", "foo.localhost", "metadata.google.internal", "db.internal"}
	for _, h := range blocked {
		if !isBlockedHostname(h) {
			t.Errorf("%s should be blocked", h)
		}
	}
	if isBlockedHostname("example.com") || isBlockedHostname("internal.example.com") {
		t.Fatal("public hosts must not be blocked by suffix")
	}
}
