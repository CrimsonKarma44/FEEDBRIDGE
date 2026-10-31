import { describe, expect, it, vi } from 'vitest';
import { ApiError, createLinkToken, detect } from '../utils/api';

const API = 'https://feedbridge.example';

function reply(status: number, body: unknown): typeof fetch {
  return vi.fn(async () => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }));
}

describe('detect', () => {
  it('calls GET /v1/detect with the encoded page URL and maps feeds', async () => {
    const f = reply(200, {
      url: 'https://www.youtube.com/@golang',
      feeds: [
        { url: 'https://www.youtube.com/feeds/videos.xml?channel_id=X', title: 'Go', type: 'atom', source: 'platform:youtube' },
        { url: 'https://example.com/odd', type: 'xml' },
        { title: 'no url' },
      ],
    });
    const feeds = await detect(API, 'https://www.youtube.com/@golang?x=1&y=2', f);
    expect(f).toHaveBeenCalledWith(
      `${API}/v1/detect?url=https%3A%2F%2Fwww.youtube.com%2F%40golang%3Fx%3D1%26y%3D2`,
      expect.objectContaining({ method: 'GET' }),
    );
    expect(feeds).toEqual([
      { url: 'https://www.youtube.com/feeds/videos.xml?channel_id=X', title: 'Go', type: 'atom', source: 'platform:youtube' },
      { url: 'https://example.com/odd', title: '', type: 'unknown', source: 'backend' },
    ]);
  });

  it('treats 404 as no feeds', async () => {
    await expect(detect(API, 'https://example.com', reply(404, { code: 5, message: 'no feeds found' }))).resolves.toEqual([]);
  });

  it('turns 429 into a friendly rate-limit error', async () => {
    const err = await detect(API, 'https://example.com', reply(429, {})).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(429);
    expect(err.message).toMatch(/too many requests/i);
  });

  it('passes the server message through for other errors', async () => {
    const err = await detect(API, 'ftp://x', reply(400, { code: 3, message: 'invalid url: only http and https urls are supported' })).catch((e) => e);
    expect(err.message).toBe('Invalid url: only http and https urls are supported');
  });

  it('reports network failures', async () => {
    const f = vi.fn(async () => {
      throw new TypeError('Failed to fetch');
    });
    const err = await detect(API, 'https://example.com', f as unknown as typeof fetch).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toMatch(/can't reach feedbridge/i);
  });
});

describe('createLinkToken', () => {
  it('POSTs the feed URL and returns the token', async () => {
    const f = reply(200, { token: 'nZRmI-9Ngxq3', expiresAt: '2026-10-31T13:05:10Z' });
    await expect(createLinkToken(API, 'https://go.dev/blog/feed.atom', f)).resolves.toBe('nZRmI-9Ngxq3');
    expect(f).toHaveBeenCalledWith(
      `${API}/v1/link-tokens`,
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ url: 'https://go.dev/blog/feed.atom' }) }),
    );
  });

  it('rejects a reply without a token', async () => {
    await expect(createLinkToken(API, 'https://go.dev/blog/feed.atom', reply(200, {}))).rejects.toBeInstanceOf(ApiError);
  });
});
