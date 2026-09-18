package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func datedItem(title string, ts time.Time) *feedpb.GetFeedsResponse_Feed {
	item := feedItem(title, "https://example.com/"+title)
	item.PublishedAt = timestamppb.New(ts)
	return item
}

func TestFreshItems_SkipsOldAndUndated(t *testing.T) {
	cursor := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	undated := feedItem("undated", "https://example.com/u")
	undated.PublishedAt = nil
	old := datedItem("old", cursor)
	newer := datedItem("new", cursor.Add(time.Hour))
	newest := datedItem("newest", cursor.Add(2*time.Hour))

	fresh, max := freshItems([]*feedpb.GetFeedsResponse_Feed{newest, newer, old, undated}, cursor)
	if len(fresh) != 2 || fresh[0].GetTitle() != "newest" || fresh[1].GetTitle() != "new" {
		t.Fatalf("fresh = %v", titles(fresh))
	}
	if !max.Equal(cursor.Add(2 * time.Hour)) {
		t.Fatalf("newest = %v", max)
	}
}

func TestOldestFirst_ReversesNewestFirstBatch(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	items := []*feedpb.GetFeedsResponse_Feed{
		datedItem("t5", t1.Add(4*time.Hour)),
		datedItem("t4", t1.Add(3*time.Hour)),
		datedItem("t3", t1.Add(2*time.Hour)),
		datedItem("t2", t1.Add(time.Hour)),
		datedItem("t1", t1),
	}
	orig := titles(items)
	got := oldestFirst(items)
	want := []string{"t1", "t2", "t3", "t4", "t5"}
	if fmt.Sprint(titles(got)) != fmt.Sprint(want) {
		t.Fatalf("oldestFirst = %v, want %v", titles(got), want)
	}
	if fmt.Sprint(titles(items)) != fmt.Sprint(orig) {
		t.Fatalf("oldestFirst mutated input: %v", titles(items))
	}
}

func TestOldestFirst_EmptyAndSingle(t *testing.T) {
	if got := oldestFirst(nil); len(got) != 0 {
		t.Fatalf("nil: %v", got)
	}
	one := []*feedpb.GetFeedsResponse_Feed{datedItem("only", time.Now())}
	if titles(oldestFirst(one))[0] != "only" {
		t.Fatal("single item should round-trip")
	}
}

func TestCursorAfterPartial_StopsAtLastSentOldestFirst(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newestFirst := []*feedpb.GetFeedsResponse_Feed{
		datedItem("t5", t1.Add(4*time.Hour)),
		datedItem("t4", t1.Add(3*time.Hour)),
		datedItem("t3", t1.Add(2*time.Hour)),
		datedItem("t2", t1.Add(time.Hour)),
		datedItem("t1", t1),
	}
	fresh, newest := freshItems(newestFirst, time.Time{})
	sent := oldestFirst(fresh)

	got := cursorAfterPartial(sent, 2, newest)
	want := t1.Add(time.Hour) // t2, the 2nd-oldest
	if !got.Equal(want) {
		t.Fatalf("cursor = %v, want %v", got, want)
	}
	for _, item := range sent[2:] {
		if !publishedAt(item).After(got) {
			t.Fatalf("unsent %s ts=%v not after cursor %v", item.GetTitle(), publishedAt(item), got)
		}
	}
}

func TestCursorAfterPartial_FullSuccessUsesNewest(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	items := []*feedpb.GetFeedsResponse_Feed{
		datedItem("t2", t1.Add(time.Hour)),
		datedItem("t1", t1),
	}
	fresh, newest := freshItems(items, time.Time{})
	sent := oldestFirst(fresh)
	got := cursorAfterPartial(sent, len(sent), newest)
	if !got.Equal(newest) {
		t.Fatalf("cursor = %v, want newest %v", got, newest)
	}
}

func TestCursorAfterPartial_ZeroDeliveredKeepsZero(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sent := oldestFirst([]*feedpb.GetFeedsResponse_Feed{
		datedItem("t2", t1.Add(time.Hour)),
		datedItem("t1", t1),
	})
	got := cursorAfterPartial(sent, 0, t1.Add(time.Hour))
	if !got.IsZero() {
		t.Fatalf("cursor = %v, want zero", got)
	}
}

func TestCursorAfterPartial_UndatedLastSentFallsBackToNewest(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	undated := feedItem("undated", "https://example.com/u")
	undated.PublishedAt = nil
	sent := []*feedpb.GetFeedsResponse_Feed{undated, datedItem("newer", t1.Add(time.Hour))}
	newest := t1.Add(time.Hour)
	got := cursorAfterPartial(sent, 1, newest)
	if !got.Equal(newest) {
		t.Fatalf("cursor = %v, want newest %v", got, newest)
	}
}

func TestCursorAfterPartial_NewestFirstWouldDropOlderItems(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newestFirst := []*feedpb.GetFeedsResponse_Feed{
		datedItem("t5", t1.Add(4*time.Hour)),
		datedItem("t4", t1.Add(3*time.Hour)),
		datedItem("t3", t1.Add(2*time.Hour)),
		datedItem("t2", t1.Add(time.Hour)),
		datedItem("t1", t1),
	}
	_, newest := freshItems(newestFirst, time.Time{})
	bugCursor := cursorAfterPartial(newestFirst, 2, newest)
	if !bugCursor.Equal(t1.Add(3 * time.Hour)) {
		t.Fatalf("sanity: newest-first cursor = %v", bugCursor)
	}
	dropped := 0
	for _, item := range newestFirst {
		if !publishedAt(item).After(bugCursor) {
			dropped++
		}
	}
	if dropped < 3 {
		t.Fatalf("expected the unfixed order to skip older items, dropped=%d", dropped)
	}
}

func TestFreshItems_EmptyAndAllOld(t *testing.T) {
	cursor := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	fresh, newest := freshItems(nil, cursor)
	if len(fresh) != 0 || !newest.IsZero() {
		t.Fatalf("empty input: fresh=%v newest=%v", fresh, newest)
	}
	old := []*feedpb.GetFeedsResponse_Feed{
		datedItem("a", cursor),
		datedItem("b", cursor.Add(-time.Hour)),
	}
	fresh, newest = freshItems(old, cursor)
	if len(fresh) != 0 || !newest.IsZero() {
		t.Fatalf("all old: fresh=%v newest=%v", titles(fresh), newest)
	}
}

func titles(items []*feedpb.GetFeedsResponse_Feed) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.GetTitle()
	}
	return out
}
