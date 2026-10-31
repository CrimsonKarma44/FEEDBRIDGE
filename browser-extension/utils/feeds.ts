// Feed types match what the backend's rss_detector reports.
export type FeedType = 'rss' | 'atom' | 'json' | 'unknown';

export interface DetectedFeed {
  url: string;
  title: string;
  type: FeedType;
  /** 'page' when read from the page's <link> tags, otherwise the backend's source label. */
  source: string;
}

export const GROUPS: { type: FeedType; label: string }[] = [
  { type: 'rss', label: 'RSS' },
  { type: 'atom', label: 'Atom' },
  { type: 'json', label: 'JSON Feed' },
  { type: 'unknown', label: 'Other' },
];

/** Groups feeds in display order, leaving out empty groups. */
export function groupFeeds(feeds: DetectedFeed[]): { type: FeedType; label: string; feeds: DetectedFeed[] }[] {
  return GROUPS.map((g) => ({ ...g, feeds: feeds.filter((f) => f.type === g.type) })).filter(
    (g) => g.feeds.length > 0,
  );
}

export function asFeedType(value: string | undefined): FeedType {
  return value === 'rss' || value === 'atom' || value === 'json' ? value : 'unknown';
}

// Sites whose feeds aren't in the page's <link> tags but that the backend
// knows how to resolve. On these, a click asks the backend straight away.
const KNOWN_PLATFORM_HOSTS = ['youtube.com', 'youtu.be', 'github.com'];

export function isKnownPlatform(pageUrl: string): boolean {
  let host: string;
  try {
    host = new URL(pageUrl).hostname.toLowerCase();
  } catch {
    return false;
  }
  return KNOWN_PLATFORM_HOSTS.some((h) => host === h || host.endsWith(`.${h}`));
}

/** True for pages the extension can scan: plain http(s) pages. */
export function isScannable(pageUrl: string | undefined): boolean {
  return !!pageUrl && /^https?:\/\//i.test(pageUrl);
}

/** "example.com/blog/feed.xml" style label for a feed URL. */
export function shortUrl(raw: string, max = 48): string {
  let s = raw.replace(/^https?:\/\//i, '').replace(/\/$/, '');
  if (s.length > max) s = `${s.slice(0, max - 1)}…`;
  return s;
}
