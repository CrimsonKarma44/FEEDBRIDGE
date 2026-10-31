import { asFeedType, type DetectedFeed } from './feeds';

const DETECT_TIMEOUT_MS = 35_000; // backend allows detection up to 30 s
const TOKEN_TIMEOUT_MS = 10_000;

/** The backend refused or failed; message is safe to show. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

type Fetch = typeof fetch;

/**
 * Asks the FeedBridge backend for a page's feeds. Resolves to [] when the
 * backend found none (HTTP 404).
 */
export async function detect(apiOrigin: string, pageUrl: string, fetchImpl: Fetch = fetch): Promise<DetectedFeed[]> {
  const url = `${apiOrigin}/v1/detect?url=${encodeURIComponent(pageUrl)}`;
  const res = await call(fetchImpl, url, { method: 'GET' }, DETECT_TIMEOUT_MS);
  if (res.status === 404) return [];
  const body = await readJson(res);
  const feeds: unknown[] = Array.isArray(body?.feeds) ? body.feeds : [];
  return feeds.filter(isRawFeed).map((f) => ({
    url: f.url,
    title: typeof f.title === 'string' ? f.title : '',
    type: asFeedType(typeof f.type === 'string' ? f.type : undefined),
    source: typeof f.source === 'string' && f.source ? f.source : 'backend',
  }));
}

interface RawFeed {
  url: string;
  title?: unknown;
  type?: unknown;
  source?: unknown;
}

function isRawFeed(f: unknown): f is RawFeed {
  return !!f && typeof f === 'object' && typeof (f as { url?: unknown }).url === 'string' && (f as RawFeed).url !== '';
}

/** Gets a single-use, 15-minute token that maps to feedUrl for the Telegram deep link. */
export async function createLinkToken(apiOrigin: string, feedUrl: string, fetchImpl: Fetch = fetch): Promise<string> {
  const res = await call(
    fetchImpl,
    `${apiOrigin}/v1/link-tokens`,
    { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url: feedUrl }) },
    TOKEN_TIMEOUT_MS,
  );
  const body = await readJson(res);
  if (typeof body?.token !== 'string' || !body.token) throw new ApiError('The server sent an unexpected reply.', res.status);
  return body.token;
}

async function call(fetchImpl: Fetch, url: string, init: RequestInit, timeoutMs: number): Promise<Response> {
  let res: Response;
  try {
    res = await fetchImpl(url, { ...init, signal: AbortSignal.timeout(timeoutMs) });
  } catch (err) {
    if (err instanceof DOMException && (err.name === 'TimeoutError' || err.name === 'AbortError')) {
      throw new ApiError('FeedBridge took too long to answer. Try again.', 0);
    }
    throw new ApiError("Can't reach FeedBridge. Check your connection.", 0);
  }
  if (res.ok || res.status === 404) return res;
  if (res.status === 429) throw new ApiError('Too many requests. Wait a few seconds and try again.', 429);
  const body = await readJson(res).catch(() => null);
  const msg = typeof body?.message === 'string' && body.message ? body.message : `FeedBridge returned an error (${res.status}).`;
  throw new ApiError(capitalize(msg), res.status);
}

async function readJson(res: Response): Promise<any> {
  try {
    return await res.json();
  } catch {
    throw new ApiError('The server sent an unexpected reply.', res.status);
  }
}

function capitalize(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}
