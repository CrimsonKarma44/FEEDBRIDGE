import { browser } from 'wxt/browser';

export interface Settings {
  /** Origin of the FeedBridge API, e.g. https://feedbridge.duckdns.org */
  apiOrigin: string;
  /** Telegram bot username without the @ */
  botUsername: string;
}

// Keep apiOrigin in step with host_permissions in wxt.config.ts.
export const DEFAULT_SETTINGS: Settings = {
  apiOrigin: 'https://feedbridge.duckdns.org',
  botUsername: 'feed_bridge_bot',
};

/**
 * Normalizes an API address to an origin. HTTPS is required, except for
 * localhost while developing. Throws with a message for the options page.
 */
export function parseApiOrigin(raw: string): string {
  let u: URL;
  try {
    u = new URL(raw.trim());
  } catch {
    throw new Error('Enter a full address, like https://feedbridge.example.com');
  }
  const local = u.hostname === 'localhost' || u.hostname === '127.0.0.1';
  if (u.protocol !== 'https:' && !(local && u.protocol === 'http:')) {
    throw new Error('The address must start with https://');
  }
  return u.origin;
}

/** Telegram usernames: 5–32 letters, digits or underscores. A leading @ is dropped. */
export function parseBotUsername(raw: string): string {
  const name = raw.trim().replace(/^@/, '');
  if (!/^[A-Za-z0-9_]{5,32}$/.test(name)) {
    throw new Error('A bot username has 5 to 32 letters, digits or underscores.');
  }
  return name;
}

export async function loadSettings(): Promise<Settings> {
  const stored = (await browser.storage.sync.get(Object.keys(DEFAULT_SETTINGS) as (keyof Settings)[])) as Partial<Settings>;
  return { ...DEFAULT_SETTINGS, ...stored };
}

export async function saveSettings(s: Settings): Promise<void> {
  await browser.storage.sync.set({ ...s });
}

/** Host-permission pattern for an API origin. */
export function originPattern(origin: string): string {
  return `${origin}/*`;
}
