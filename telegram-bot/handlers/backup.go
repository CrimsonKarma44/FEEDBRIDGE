package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	subpb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/subscription"
	"github.com/CrimsonKarma44/FEEDBRIDGE/telegram-bot/grpcclient"
	tele "gopkg.in/telebot.v4"
)

const (
	backupVersion   = 1
	backupFileName  = "feedbridge-backup.json"
	maxBackupBytes  = 1 << 20
	maxBackupFeeds  = 200
	maxFailureLines = 15
	restoreWait     = 10 * time.Minute
)

// backupFile is the document /backup sends and /restore accepts.
// IntervalSeconds is omitted when the chat has no feeds. Enabled is always set on export.
type backupFile struct {
	Version         int          `json:"version"`
	ExportedAt      time.Time    `json:"exported_at"`
	IntervalSeconds *int         `json:"interval_seconds,omitempty"`
	Feeds           []backupFeed `json:"feeds"`
}

type backupFeed struct {
	URL     string `json:"url"`
	Enabled *bool  `json:"enabled"`
}

type knownFeed struct {
	URL             string
	Enabled         bool
	IntervalSeconds int
}

type actionKind int

const (
	actionSubscribe actionKind = iota + 1
	actionEnable
	actionDisable
	actionUnchanged
	actionInvalid
)

type plannedFeed struct {
	URL       string
	StoredURL string
	Enabled   bool
	Kind      actionKind
	Detail    string
}

type restorePlan struct {
	Feeds           []plannedFeed
	Interval        time.Duration
	SetInterval     bool
	IntervalClamped bool
}

type restoreResult struct {
	added           int
	already         int
	enabled         int
	disabled        int
	failures        []string
	interval        time.Duration
	setInterval     bool
	intervalClamped bool
	intervalFailed  string
	empty           bool
}

type waitState int

const (
	waitNone waitState = iota
	waitReady
	waitExpired
)

// restoreSlots remembers chats that sent /restore and are waiting for a file.
// The map is in memory and is empty again after a restart.
type restoreSlots struct {
	mu    sync.Mutex
	until map[int64]time.Time
}

func (s *restoreSlots) arm(chatID int64, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.until == nil {
		s.until = make(map[int64]time.Time)
	}
	s.until[chatID] = now.Add(restoreWait)
}

func (s *restoreSlots) state(chatID int64, now time.Time) waitState {
	s.mu.Lock()
	defer s.mu.Unlock()
	deadline, ok := s.until[chatID]
	if !ok {
		return waitNone
	}
	if !now.Before(deadline) {
		delete(s.until, chatID)
		return waitExpired
	}
	return waitReady
}

func (s *restoreSlots) clear(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.until, chatID)
}

func knownFromSubs(subs []*subpb.Subscription) []knownFeed {
	out := make([]knownFeed, 0, len(subs))
	for _, sub := range subs {
		if sub == nil {
			continue
		}
		out = append(out, knownFeed{
			URL:             sub.GetUrl(),
			Enabled:         sub.GetEnabled(),
			IntervalSeconds: int(sub.GetIntervalSeconds()),
		})
	}
	return out
}

func buildBackup(subs []knownFeed, now time.Time) backupFile {
	file := backupFile{
		Version:    backupVersion,
		ExportedAt: now.UTC().Truncate(time.Second),
		Feeds:      make([]backupFeed, 0, len(subs)),
	}
	for _, sub := range subs {
		enabled := sub.Enabled
		file.Feeds = append(file.Feeds, backupFeed{URL: sub.URL, Enabled: &enabled})
	}
	file.IntervalSeconds = dominantInterval(subs)
	return file
}

func dominantInterval(subs []knownFeed) *int {
	if len(subs) == 0 {
		return nil
	}
	counts := make(map[int]int, len(subs))
	for _, sub := range subs {
		counts[sub.IntervalSeconds]++
	}
	best, bestN := 0, -1
	for sec, n := range counts {
		if n > bestN || (n == bestN && sec < best) {
			best, bestN = sec, n
		}
	}
	return &best
}

func parseBackup(data []byte) (backupFile, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return backupFile{}, fmt.Errorf("it is empty")
	}
	var file backupFile
	if err := json.Unmarshal(data, &file); err != nil {
		return backupFile{}, fmt.Errorf("it is not valid JSON")
	}
	if file.Version != backupVersion {
		return backupFile{}, fmt.Errorf("it is version %d, and only version 1 is supported", file.Version)
	}
	if len(file.Feeds) > maxBackupFeeds {
		return backupFile{}, fmt.Errorf("it lists %d feeds, and the maximum is %d", len(file.Feeds), maxBackupFeeds)
	}
	if file.Feeds == nil {
		file.Feeds = []backupFeed{}
	}
	return file, nil
}

func planRestore(existing []knownFeed, file backupFile) restorePlan {
	byURL := make(map[string]knownFeed, len(existing)*2)
	add := func(key string, feed knownFeed) {
		if key == "" {
			return
		}
		if _, ok := byURL[key]; !ok {
			byURL[key] = feed
		}
	}
	for _, sub := range existing {
		add(sub.URL, sub)
		if key, err := NormalizeURL(sub.URL); err == nil {
			add(key, sub)
		}
	}

	plan := restorePlan{Feeds: make([]plannedFeed, 0, len(file.Feeds))}
	seen := make(map[string]struct{}, len(file.Feeds))
	for _, feed := range file.Feeds {
		raw := strings.TrimSpace(feed.URL)
		key, err := NormalizeURL(raw)
		if err != nil {
			plan.Feeds = append(plan.Feeds, plannedFeed{
				URL:    raw,
				Kind:   actionInvalid,
				Detail: err.Error(),
			})
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if feed.Enabled == nil {
			plan.Feeds = append(plan.Feeds, plannedFeed{
				URL:    key,
				Kind:   actionInvalid,
				Detail: "enabled is required",
			})
			continue
		}
		if have, ok := byURL[key]; ok {
			item := plannedFeed{URL: key, StoredURL: have.URL, Enabled: *feed.Enabled}
			switch {
			case have.Enabled == *feed.Enabled:
				item.Kind = actionUnchanged
			case *feed.Enabled:
				item.Kind = actionEnable
			default:
				item.Kind = actionDisable
			}
			plan.Feeds = append(plan.Feeds, item)
			continue
		}
		plan.Feeds = append(plan.Feeds, plannedFeed{
			URL:     key,
			Enabled: *feed.Enabled,
			Kind:    actionSubscribe,
		})
	}

	if file.IntervalSeconds != nil {
		plan.Interval, plan.IntervalClamped = clampIntervalSeconds(*file.IntervalSeconds)
		plan.SetInterval = true
	}
	return plan
}

func clampIntervalSeconds(sec int) (time.Duration, bool) {
	minSec := int(minInterval / time.Second)
	maxSec := int(maxInterval / time.Second)
	clamped := false
	if sec < minSec {
		sec = minSec
		clamped = true
	}
	if sec > maxSec {
		sec = maxSec
		clamped = true
	}
	return time.Duration(sec) * time.Second, clamped
}

func isRestoreCaption(caption, botUser string) bool {
	caption = strings.TrimSpace(caption)
	if caption == "" {
		return false
	}
	field := strings.ToLower(strings.Fields(caption)[0])
	mention := ""
	if i := strings.IndexByte(field, '@'); i >= 0 {
		mention = field[i+1:]
		field = field[:i]
	}
	if field != "/restore" {
		return false
	}
	if mention == "" {
		return true
	}
	return botUser != "" && strings.EqualFold(mention, botUser)
}

func failureLine(url, detail string) string {
	if strings.TrimSpace(url) == "" {
		return esc(detail)
	}
	return esc(url) + " — " + esc(detail)
}

func formatRestoreSummary(r restoreResult) string {
	if r.empty {
		return "Nothing to restore. The file has no feeds."
	}
	var b strings.Builder
	b.WriteString("Restore finished.")
	fmt.Fprintf(&b, "\nAdded: %d.", r.added)
	fmt.Fprintf(&b, "\nAlready subscribed: %d.", r.already)
	fmt.Fprintf(&b, "\nEnabled: %d.", r.enabled)
	fmt.Fprintf(&b, "\nDisabled: %d.", r.disabled)
	fmt.Fprintf(&b, "\nFailed: %d.", len(r.failures))
	limit := len(r.failures)
	extra := 0
	if limit > maxFailureLines {
		extra = limit - maxFailureLines
		limit = maxFailureLines
	}
	for _, line := range r.failures[:limit] {
		b.WriteString("\n- ")
		b.WriteString(line)
	}
	if extra > 0 {
		fmt.Fprintf(&b, "\n- and %d more", extra)
	}
	if r.intervalFailed != "" {
		b.WriteString("\n")
		b.WriteString(r.intervalFailed)
	} else if r.setInterval {
		fmt.Fprintf(&b, "\nAll feeds in this chat now refresh every %s.", r.interval)
		if r.intervalClamped {
			fmt.Fprintf(&b, "\n<i>Clamped to the allowed range (%s - %s).</i>", minInterval, maxInterval)
		}
	}
	return b.String()
}

func (h *FeedHandler) Backup(c tele.Context) error {
	if adminGate(c) {
		return nil
	}
	if c.Chat() == nil {
		return nil
	}
	platform, ext := telegramDest(c.Chat().ID)
	subs, err := h.API.ListSubscriptions(context.Background(), platform, ext)
	if err != nil {
		log.Println("backup list failed:", err)
		return c.Send("Could not load your feeds.")
	}
	file := buildBackup(knownFromSubs(subs), time.Now())
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		log.Println("backup encode failed:", err)
		return c.Send("Could not build the backup file.")
	}
	raw = append(raw, '\n')

	n := len(file.Feeds)
	caption := fmt.Sprintf("Backup of %d feed", n)
	if n != 1 {
		caption += "s"
	}
	caption += ".\nTo restore: send this file with the caption /restore, or send /restore and then the file within 10 minutes."
	doc := &tele.Document{
		File:     tele.FromReader(bytes.NewReader(raw)),
		FileName: backupFileName,
		Caption:  caption,
	}
	return c.Send(doc)
}

func (h *FeedHandler) Restore(c tele.Context) error {
	if adminGate(c) {
		return nil
	}
	if c.Chat() == nil {
		return nil
	}
	h.restores.arm(c.Chat().ID, time.Now())
	return c.Send("Send the backup file here within 10 minutes.\nYou can also send the file with the caption /restore.")
}

func (h *FeedHandler) OnDocument(c tele.Context) error {
	msg := c.Message()
	if msg == nil || msg.Document == nil || c.Chat() == nil {
		return nil
	}
	botUser := ""
	if bot, ok := c.Bot().(*tele.Bot); ok && bot != nil && bot.Me != nil {
		botUser = bot.Me.Username
	}
	caption := isRestoreCaption(c.Text(), botUser)
	wait := h.restores.state(c.Chat().ID, time.Now())
	if !caption && wait == waitNone {
		return nil
	}
	if !caption && wait == waitExpired {
		if !IsAdmin(c) {
			return nil
		}
		return c.Send("That restore request expired. Send /restore, then the backup file, within 10 minutes.")
	}
	if !caption && wait == waitReady && !IsAdmin(c) {
		return nil
	}
	if adminGate(c) {
		return nil
	}
	h.restores.clear(c.Chat().ID)
	return h.restoreDocument(c, msg.Document)
}

func (h *FeedHandler) restoreDocument(c tele.Context, doc *tele.Document) error {
	if doc.FileSize > maxBackupBytes {
		return c.Send("That file is too large. A backup must be 1 MB or smaller.")
	}
	reader, err := c.Bot().File(&doc.File)
	if err != nil {
		log.Printf("backup download failed: %v", err)
		return c.Send("Could not download that file. Try sending it again.")
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxBackupBytes+1))
	if err != nil {
		log.Printf("backup read failed: %v", err)
		return c.Send("Could not read that file. Try sending it again.")
	}
	if len(data) > maxBackupBytes {
		return c.Send("That file is too large. A backup must be 1 MB or smaller.")
	}
	file, err := parseBackup(data)
	if err != nil {
		return c.Send("That file is not a FEEDBRIDGE backup. " + esc(err.Error()))
	}
	return h.applyBackup(c, file)
}

func (h *FeedHandler) applyBackup(c tele.Context, file backupFile) error {
	platform, ext := telegramDest(c.Chat().ID)
	subs, err := h.API.ListSubscriptions(context.Background(), platform, ext)
	if err != nil {
		log.Println("restore list failed:", err)
		return c.Send("Could not load your feeds.")
	}
	known := knownFromSubs(subs)
	plan := planRestore(known, file)
	if len(file.Feeds) > 0 {
		if err := c.Send(fmt.Sprintf("Restoring %d feeds…", len(file.Feeds))); err != nil {
			return err
		}
	}

	result := restoreResult{
		setInterval:     plan.SetInterval,
		interval:        plan.Interval,
		intervalClamped: plan.IntervalClamped,
		empty:           len(file.Feeds) == 0 && !plan.SetInterval,
	}
	for _, item := range plan.Feeds {
		switch item.Kind {
		case actionInvalid:
			result.failures = append(result.failures, failureLine(item.URL, item.Detail))
		case actionUnchanged:
			result.already++
		case actionEnable, actionDisable:
			want := item.Kind == actionEnable
			if err := h.API.SetEnabledByURL(context.Background(), platform, ext, item.StoredURL, want); err != nil {
				log.Printf("restore enabled %s: %v", item.StoredURL, err)
				result.failures = append(result.failures, failureLine(item.URL, grpcclient.FriendlyError(err)))
				continue
			}
			if want {
				result.enabled++
			} else {
				result.disabled++
			}
		case actionSubscribe:
			res, err := h.API.Subscribe(context.Background(), platform, ext, item.URL)
			if err != nil {
				log.Printf("restore subscribe %s: %v", item.URL, err)
				result.failures = append(result.failures, failureLine(item.URL, grpcclient.FriendlyError(err)))
				continue
			}
			canonical := item.URL
			if res.GetSubscription() != nil && res.GetSubscription().GetUrl() != "" {
				canonical = res.GetSubscription().GetUrl()
			}
			if !res.GetCreated() {
				result.already++
				log.Printf("restore subscribe matched an existing feed for %s", item.URL)
				if !item.Enabled {
					if err := h.API.SetEnabledByURL(context.Background(), platform, ext, canonical, false); err != nil {
						log.Printf("restore disable existing %s: %v", canonical, err)
						result.failures = append(result.failures, failureLine(canonical, "already subscribed, but could not disable it"))
					} else {
						result.disabled++
					}
				}
				continue
			}
			result.added++
			if !item.Enabled {
				if err := h.API.SetEnabledByURL(context.Background(), platform, ext, canonical, false); err != nil {
					log.Printf("restore disable new %s: %v", canonical, err)
					result.failures = append(result.failures, failureLine(canonical, "added, but could not disable it"))
				} else {
					result.disabled++
				}
			}
		}
	}

	if plan.SetInterval {
		if len(known) == 0 && result.added == 0 {
			result.intervalFailed = "Interval was not changed because this chat has no feeds."
			result.setInterval = false
		} else if err := h.API.SetInterval(context.Background(), platform, ext, plan.Interval); err != nil {
			log.Println("restore interval failed:", err)
			if grpcclient.IsNotFound(err) {
				result.intervalFailed = "Interval was not changed because this chat has no feeds."
			} else {
				result.intervalFailed = "Could not update the refresh interval."
			}
			result.setInterval = false
		}
	}
	return c.Send(formatRestoreSummary(result))
}
