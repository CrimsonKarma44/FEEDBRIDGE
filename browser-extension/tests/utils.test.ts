import { describe, expect, it } from 'vitest';
import { groupFeeds, isKnownPlatform, isScannable, shortUrl, type DetectedFeed } from '../utils/feeds';
import { parseApiOrigin, parseBotUsername } from '../utils/settings';
import { addFeedCommand, botLink, deepLink } from '../utils/telegram';

describe('telegram links', () => {
  it('builds start and startgroup deep links', () => {
    expect(deepLink('feed_bridge_bot', 'nZRmI-9Ngxq3', 'chat')).toBe('https://t.me/feed_bridge_bot?start=add_nZRmI-9Ngxq3');
    expect(deepLink('feed_bridge_bot', 'nZRmI-9Ngxq3', 'group')).toBe('https://t.me/feed_bridge_bot?startgroup=add_nZRmI-9Ngxq3');
  });
  it('keeps the start payload within Telegram’s 64 characters', () => {
    const payload = new URL(deepLink('feed_bridge_bot', 'nZRmI-9Ngxq3', 'chat')).searchParams.get('start')!;
    expect(payload.length).toBeLessThanOrEqual(64);
    expect(payload).toMatch(/^[A-Za-z0-9_-]+$/);
  });
  it('builds the fallback command and bot link', () => {
    expect(addFeedCommand('https://go.dev/blog/feed.atom')).toBe('/addfeed https://go.dev/blog/feed.atom');
    expect(botLink('feed_bridge_bot')).toBe('https://t.me/feed_bridge_bot');
  });
});

describe('feeds helpers', () => {
  const f = (type: DetectedFeed['type'], url: string): DetectedFeed => ({ url, title: '', type, source: 'page' });

  it('groups in RSS, Atom, JSON, Other order and drops empty groups', () => {
    const groups = groupFeeds([f('unknown', 'u'), f('atom', 'a1'), f('rss', 'r'), f('atom', 'a2')]);
    expect(groups.map((g) => [g.label, g.feeds.map((x) => x.url)])).toEqual([
      ['RSS', ['r']],
      ['Atom', ['a1', 'a2']],
      ['Other', ['u']],
    ]);
  });

  it('recognises known platforms only by host', () => {
    expect(isKnownPlatform('https://www.youtube.com/@golang')).toBe(true);
    expect(isKnownPlatform('https://youtu.be/abc')).toBe(true);
    expect(isKnownPlatform('https://github.com/golang/go')).toBe(true);
    expect(isKnownPlatform('https://notyoutube.com/x')).toBe(false);
    expect(isKnownPlatform('https://example.com/?u=youtube.com')).toBe(false);
    expect(isKnownPlatform('not a url')).toBe(false);
  });

  it('only scans http(s) pages', () => {
    expect(isScannable('https://example.com')).toBe(true);
    expect(isScannable('http://example.com')).toBe(true);
    expect(isScannable('chrome://extensions')).toBe(false);
    expect(isScannable('about:blank')).toBe(false);
    expect(isScannable(undefined)).toBe(false);
  });

  it('shortens URLs for display', () => {
    expect(shortUrl('https://example.com/feed/')).toBe('example.com/feed');
    expect(shortUrl('https://example.com/' + 'a'.repeat(80), 20)).toHaveLength(20);
  });
});

describe('settings validation', () => {
  it('accepts https origins and strips paths', () => {
    expect(parseApiOrigin(' https://feedbridge.duckdns.org/v1/ ')).toBe('https://feedbridge.duckdns.org');
  });
  it('allows http only for localhost', () => {
    expect(parseApiOrigin('http://localhost:8080')).toBe('http://localhost:8080');
    expect(() => parseApiOrigin('http://feedbridge.example')).toThrow(/https/);
    expect(() => parseApiOrigin('feedbridge')).toThrow(/full address/);
  });
  it('validates bot usernames', () => {
    expect(parseBotUsername('@feed_bridge_bot')).toBe('feed_bridge_bot');
    expect(() => parseBotUsername('abc')).toThrow();
    expect(() => parseBotUsername('bad name!')).toThrow();
  });
});
