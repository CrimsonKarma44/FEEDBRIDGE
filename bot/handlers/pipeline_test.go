package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	tele "gopkg.in/telebot.v4"
)

type sendCall struct {
	text string
	mode string
}

type fakeSender struct {
	calls    []sendCall
	failHTML bool
	failAll  bool
}

func (f *fakeSender) Send(_ tele.Recipient, what any, opts ...any) (*tele.Message, error) {
	text, _ := what.(string)
	mode := ""
	for _, o := range opts {
		if so, ok := o.(*tele.SendOptions); ok && so != nil {
			mode = string(so.ParseMode)
		}
	}
	if f.failAll {
		return nil, fmt.Errorf("telegram: Forbidden: bot was blocked by the user (403)")
	}
	if f.failHTML && mode == string(tele.ModeHTML) {
		return nil, fmt.Errorf(`telegram: Bad Request: can't parse entities: Can't find end tag corresponding to start tag "a" (400)`)
	}
	f.calls = append(f.calls, sendCall{text: text, mode: mode})
	return &tele.Message{Text: text}, nil
}

func TestSendDigest_RetriesPlainOnParseError(t *testing.T) {
	bot := &fakeSender{failHTML: true}
	items := []*feedpb.GetFeedsResponse_Feed{
		feedItem("One", "https://example.com/1"),
		feedItem("Two", "https://example.com/2"),
	}
	if err := SendDigest(bot, 1, items); err != nil {
		t.Fatalf("SendDigest: %v", err)
	}
	if len(bot.calls) != 1 {
		t.Fatalf("got %d sends, want 1 plain retry", len(bot.calls))
	}
	if bot.calls[0].mode != "" {
		t.Fatalf("retry parse mode = %q, want empty", bot.calls[0].mode)
	}
	if strings.Contains(bot.calls[0].text, "<a ") {
		t.Fatalf("plain retry still has HTML: %s", bot.calls[0].text)
	}
	if !strings.Contains(bot.calls[0].text, "One") || !strings.Contains(bot.calls[0].text, "Two") {
		t.Fatalf("plain retry missing items: %s", bot.calls[0].text)
	}
}

func TestSendItems_RetriesPlainOnParseError(t *testing.T) {
	bot := &fakeSender{failHTML: true}
	items := []*feedpb.GetFeedsResponse_Feed{
		feedItem("Hello", "https://example.com/1"),
	}
	if n := SendItems(bot, 1, items, 0); n != 1 {
		t.Fatalf("delivered %d, want 1", n)
	}
	if len(bot.calls) != 1 || bot.calls[0].mode != "" {
		t.Fatalf("calls = %+v", bot.calls)
	}
}

func TestSendDigest_StopsOnHardFailure(t *testing.T) {
	bot := &fakeSender{failAll: true}
	err := SendDigest(bot, 1, []*feedpb.GetFeedsResponse_Feed{feedItem("X", "https://example.com")})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(bot.calls) != 0 {
		t.Fatalf("unexpected successful sends: %+v", bot.calls)
	}
}

func TestSendDigest_SendsEveryPage(t *testing.T) {
	longURL := "https://example.com/" + strings.Repeat("a", 1800)
	items := make([]*feedpb.GetFeedsResponse_Feed, 12)
	for i := range items {
		items[i] = feedItem(fmt.Sprintf("Story %d", i), longURL)
	}
	pages := FormatDigestPages(items)
	if len(pages) < 2 {
		t.Fatalf("test setup: expected multiple pages, got %d", len(pages))
	}
	bot := &fakeSender{}
	if err := SendDigest(bot, 42, items); err != nil {
		t.Fatalf("SendDigest: %v", err)
	}
	if len(bot.calls) != len(pages) {
		t.Fatalf("sent %d messages, want %d pages", len(bot.calls), len(pages))
	}
	for i, c := range bot.calls {
		if c.mode != string(tele.ModeHTML) {
			t.Fatalf("page %d mode = %q", i, c.mode)
		}
		if c.text != pages[i] {
			t.Fatalf("page %d body mismatch", i)
		}
	}
}

func TestSendItems_ZeroGapDoesNotSleepLong(t *testing.T) {
	bot := &fakeSender{}
	start := time.Now()
	n := SendItems(bot, 1, []*feedpb.GetFeedsResponse_Feed{
		feedItem("A", "https://example.com/a"),
		feedItem("B", "https://example.com/b"),
	}, 0)
	if n != 2 {
		t.Fatalf("delivered %d", n)
	}
	if time.Since(start) > time.Second {
		t.Fatal("SendItems slept too long with zero gap")
	}
}
