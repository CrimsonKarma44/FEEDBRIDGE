import type { DetectedFeed } from './feeds';

/**
 * Reads feed <link> tags from the current document.
 *
 * Injected into the page with scripting.executeScript({ func: scanPage }),
 * so it must be self-contained: no imports, no closures, only DOM APIs.
 * It runs once per toolbar click and leaves nothing behind.
 */
export function scanPage(): DetectedFeed[] {
  const out: DetectedFeed[] = [];
  const seen = new Set<string>();
  const links = document.querySelectorAll<HTMLLinkElement>('link[rel][href]');

  for (const link of Array.from(links)) {
    const rel = (link.getAttribute('rel') || '').toLowerCase().split(/\s+/);
    if (!rel.includes('alternate') && !rel.includes('feed')) continue;

    const mime = ((link.getAttribute('type') || '').toLowerCase().split(';')[0] ?? '').trim();
    const title = (link.getAttribute('title') || '').trim();
    const href = link.getAttribute('href') || '';
    const looksLikeFeed = /feed|rss|atom/i.test(title + ' ' + href);

    let type: DetectedFeed['type'] | null = null;
    if (mime === 'application/rss+xml') type = 'rss';
    else if (mime === 'application/atom+xml') type = 'atom';
    else if (mime === 'application/feed+json') type = 'json';
    // Plain JSON or XML alternates are often translations or API docs; keep
    // them only when the title or URL says they're a feed.
    else if (mime === 'application/json' && looksLikeFeed) type = 'json';
    else if ((mime === 'application/xml' || mime === 'text/xml') && looksLikeFeed) type = 'unknown';
    else if (mime === '' && rel.includes('feed')) type = 'unknown';
    if (!type) continue;

    let url: string;
    try {
      url = new URL(href, document.baseURI).href;
    } catch {
      continue;
    }
    if (!/^https?:\/\//i.test(url) || seen.has(url)) continue;
    seen.add(url);
    out.push({ url, title, type, source: 'page' });
  }
  return out;
}
