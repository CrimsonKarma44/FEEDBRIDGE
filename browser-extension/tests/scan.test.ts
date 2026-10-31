// @vitest-environment jsdom
// @vitest-environment-options {"url": "https://example.com/blog/post-1"}
import { beforeEach, describe, expect, it } from 'vitest';
import { scanPage } from '../utils/scan';

// The document lives at https://example.com/blog/post-1 (see the options above).
function page(head: string) {
  document.head.innerHTML = head;
}

describe('scanPage', () => {
  beforeEach(() => {
    document.head.innerHTML = '';
  });

  it('reads RSS, Atom and JSON Feed links and resolves relative URLs', () => {
    page(`
      <link rel="alternate" type="application/rss+xml" title="Example RSS" href="/feed.xml">
      <link rel="alternate" type="application/atom+xml" title="Example Atom" href="atom.xml">
      <link rel="alternate" type="application/feed+json" href="https://example.com/feed.json">
    `);
    expect(scanPage()).toEqual([
      { url: 'https://example.com/feed.xml', title: 'Example RSS', type: 'rss', source: 'page' },
      { url: 'https://example.com/blog/atom.xml', title: 'Example Atom', type: 'atom', source: 'page' },
      { url: 'https://example.com/feed.json', title: '', type: 'json', source: 'page' },
    ]);
  });

  it('honours <base href>', () => {
    page(`<base href="https://cdn.example.org/site/"><link rel="alternate" type="application/rss+xml" href="rss">`);
    expect(scanPage().map((f) => f.url)).toEqual(['https://cdn.example.org/site/rss']);
  });

  it('ignores non-feed alternates, stylesheets and duplicates', () => {
    page(`
      <link rel="stylesheet" href="/style.css">
      <link rel="alternate" hreflang="fr" href="/fr/">
      <link rel="alternate" type="application/json" href="/wp-json/wp/v2/posts/1">
      <link rel="alternate" type="application/rss+xml" href="/feed.xml">
      <link rel="alternate" type="application/rss+xml" href="https://example.com/feed.xml">
      <link rel="alternate" type="application/rss+xml" href="javascript:alert(1)">
    `);
    expect(scanPage().map((f) => f.url)).toEqual(['https://example.com/feed.xml']);
  });

  it('keeps plain JSON/XML alternates only when they look like feeds', () => {
    page(`
      <link rel="alternate" type="application/json" title="JSON Feed" href="/feed/json">
      <link rel="alternate" type="text/xml" title="Comments RSS" href="/comments.xml">
      <link rel="alternate" type="text/xml" href="/sitemap.xml">
    `);
    expect(scanPage()).toEqual([
      { url: 'https://example.com/feed/json', title: 'JSON Feed', type: 'json', source: 'page' },
      { url: 'https://example.com/comments.xml', title: 'Comments RSS', type: 'unknown', source: 'page' },
    ]);
  });

  it('handles MIME parameters and mixed case', () => {
    page(`<link rel="Alternate" type="Application/RSS+XML; charset=utf-8" href="/rss">`);
    expect(scanPage()).toEqual([{ url: 'https://example.com/rss', title: '', type: 'rss', source: 'page' }]);
  });

  it('returns nothing for a page without feeds', () => {
    page(`<link rel="icon" href="/favicon.ico">`);
    expect(scanPage()).toEqual([]);
  });

  it('is self-contained so it can be injected with executeScript', () => {
    // executeScript serializes the function; it must not reference imports.
    expect(scanPage.toString()).not.toMatch(/__vite|import\(|require\(/);
  });
});
