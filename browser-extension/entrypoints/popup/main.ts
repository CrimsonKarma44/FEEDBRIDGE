import { browser } from 'wxt/browser';
import '~/assets/theme.css';
import './style.css';
import { ApiError, createLinkToken, detect } from '~/utils/api';
import { groupFeeds, isKnownPlatform, isScannable, shortUrl, type DetectedFeed } from '~/utils/feeds';
import { scanPage } from '~/utils/scan';
import { loadSettings, originPattern, type Settings } from '~/utils/settings';
import { addFeedCommand, botLink, deepLink, type SubscribeTarget } from '~/utils/telegram';

const content = document.getElementById('content')!;
const pageHost = document.getElementById('page-host')!;
const foundBy = document.getElementById('found-by')!;

document.getElementById('open-settings')!.addEventListener('click', () => {
  void browser.runtime.openOptionsPage();
  window.close();
});

let settings: Settings;
let tabId: number | undefined;
let pageUrl = '';

void start();

async function start() {
  settings = await loadSettings();
  const [tab] = await browser.tabs.query({ active: true, currentWindow: true });
  tabId = tab?.id;
  pageUrl = tab?.url ?? '';

  if (!tabId || !isScannable(pageUrl)) {
    showState("This page can't be scanned.", 'FeedBridge works on regular web pages (http and https).');
    return;
  }
  pageHost.textContent = new URL(pageUrl).hostname;

  // 1. The page's own <link> tags. activeTab covers this tab for this click.
  let feeds: DetectedFeed[] = [];
  try {
    const [result] = await browser.scripting.executeScript({ target: { tabId }, func: scanPage });
    feeds = (result?.result as DetectedFeed[] | undefined) ?? [];
  } catch {
    showState("This page can't be scanned.", 'The browser blocks extensions on this page.');
    return;
  }
  if (feeds.length > 0) {
    showFeeds(feeds, 'Found in this page');
    return;
  }

  // 2. Known platforms (YouTube, GitHub) keep feeds out of their <link> tags:
  //    ask the backend now, as part of this click, if we already have access.
  if (isKnownPlatform(pageUrl) && (await browser.permissions.contains({ origins: [originPattern(settings.apiOrigin)] }))) {
    await scanWithBackend();
    return;
  }

  // 3. Anything else: only when the user asks.
  showEmpty();
}

function showEmpty() {
  setBadge(0);
  foundBy.textContent = '';
  const box = el('div', 'empty');
  box.append(
    el('p', '', "This page doesn't list any feeds in its code."),
    el('p', '', 'FeedBridge can check the site more thoroughly. That sends this page’s address to FeedBridge.'),
  );
  const btn = button('Scan with FeedBridge', 'btn primary');
  btn.addEventListener('click', async () => {
    // Request access first, while the click still counts as a user gesture.
    const granted = await requestApiAccess();
    if (!granted) {
      showState('FeedBridge needs permission to contact its server.', 'Click Scan again and choose Allow.');
      return;
    }
    await scanWithBackend();
  });
  box.append(btn);
  content.replaceChildren(box);
}

async function scanWithBackend() {
  showState('Checking with FeedBridge…');
  try {
    const feeds = await detect(settings.apiOrigin, pageUrl);
    if (feeds.length === 0) {
      setBadge(0);
      showState('No feeds found.', 'FeedBridge checked this site and found no RSS, Atom or JSON feeds.');
      return;
    }
    showFeeds(feeds, 'Found by FeedBridge');
  } catch (err) {
    const box = el('div', 'empty');
    box.append(el('p', '', errorText(err)));
    const retry = button('Try again', 'btn');
    retry.addEventListener('click', () => void scanWithBackend());
    box.append(retry);
    content.replaceChildren(box);
  }
}

function showFeeds(feeds: DetectedFeed[], how: string) {
  setBadge(feeds.length);
  foundBy.textContent = `${how} · ${feeds.length} feed${feeds.length === 1 ? '' : 's'}`;
  const nodes = groupFeeds(feeds).map((g) => {
    const section = el('section', 'group');
    const h = el('h2', '', g.label);
    h.append(el('span', 'count', String(g.feeds.length)));
    const list = el('ul', 'feeds');
    list.append(...g.feeds.map(feedRow));
    section.append(h, list);
    return section;
  });
  content.replaceChildren(...nodes);
}

function feedRow(feed: DetectedFeed): HTMLLIElement {
  const li = el('li', 'feed');
  li.append(el('div', 'feed-title', feed.title || new URL(feed.url).hostname));
  li.append(el('div', 'feed-url mono', shortUrl(feed.url, 60)));

  const actions = el('div', 'feed-actions');
  const copy = button('Copy URL', 'btn');
  copy.addEventListener('click', async () => {
    await copyText(feed.url);
    flash(copy, 'Copied');
  });
  const open = button('Open', 'btn');
  open.addEventListener('click', () => void browser.tabs.create({ url: feed.url }));
  const sub = button('Subscribe', 'btn primary');
  sub.setAttribute('aria-expanded', 'false');
  actions.append(copy, open, sub);
  li.append(actions);

  let panel: HTMLElement | null = null;
  sub.addEventListener('click', () => {
    if (panel) {
      panel.remove();
      panel = null;
      sub.setAttribute('aria-expanded', 'false');
      return;
    }
    panel = subscribePanel(feed);
    li.append(panel);
    sub.setAttribute('aria-expanded', 'true');
  });
  return li;
}

function subscribePanel(feed: DetectedFeed): HTMLElement {
  const panel = el('div', 'subscribe-panel');
  panel.append(el('p', '', 'Telegram opens and asks you to confirm.'));
  const row = el('div', 'row');
  const toChat = button('My chat with the bot', 'btn primary');
  const toGroup = button('A group', 'btn');
  row.append(toChat, toGroup);
  panel.append(row);

  const go = async (target: SubscribeTarget, btn: HTMLButtonElement) => {
    toChat.disabled = toGroup.disabled = true;
    const label = btn.textContent;
    btn.textContent = 'Opening…';
    const granted = await requestApiAccess();
    try {
      if (!granted) throw new ApiError('FeedBridge needs permission to contact its server.', 0);
      const token = await createLinkToken(settings.apiOrigin, feed.url);
      await browser.tabs.create({ url: deepLink(settings.botUsername, token, target) });
      window.close();
    } catch (err) {
      btn.textContent = label;
      toChat.disabled = toGroup.disabled = false;
      panel.querySelector('.notice')?.remove();
      panel.append(fallbackNotice(feed, err));
    }
  };
  toChat.addEventListener('click', () => void go('chat', toChat));
  toGroup.addEventListener('click', () => void go('group', toGroup));
  return panel;
}

// When the backend can't make a link, the user can still paste the command.
function fallbackNotice(feed: DetectedFeed, err: unknown): HTMLElement {
  const box = el('div', 'notice');
  box.append(el('span', '', `${errorText(err)} You can add it by hand instead:`));
  const cmd = addFeedCommand(feed.url);
  box.append(el('code', '', cmd));
  const row = el('div', 'row');
  const copy = button('Copy command', 'btn');
  copy.addEventListener('click', async () => {
    await copyText(cmd);
    flash(copy, 'Copied');
  });
  const open = button('Open the bot', 'btn');
  open.addEventListener('click', () => void browser.tabs.create({ url: botLink(settings.botUsername) }));
  row.append(copy, open);
  box.append(row);
  return box;
}

/**
 * Asks for access to the API origin. Must be the first await in a click
 * handler: Firefox only shows the prompt during a user gesture. Resolves
 * true straight away when access was already granted.
 */
function requestApiAccess(): Promise<boolean> {
  return browser.permissions.request({ origins: [originPattern(settings.apiOrigin)] }).catch(() => false);
}

function setBadge(count: number) {
  if (tabId === undefined) return;
  void browser.action.setBadgeText({ tabId, text: count > 0 ? String(count) : '' });
  if (count > 0) void browser.action.setBadgeBackgroundColor({ tabId, color: '#f47a20' });
}

function showState(title: string, detail?: string) {
  const p = el('p', 'state');
  if (detail) {
    p.append(el('strong', '', title), document.createTextNode(detail));
  } else {
    p.textContent = title;
  }
  content.replaceChildren(p);
}

function errorText(err: unknown): string {
  return err instanceof ApiError ? err.message : 'Something went wrong. Try again.';
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // Older Firefox popups: fall back to a hidden textarea.
    const ta = el('textarea', '');
    ta.value = text;
    document.body.append(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }
}

function flash(btn: HTMLButtonElement, text: string) {
  const old = btn.textContent;
  btn.textContent = text;
  btn.classList.add('ok');
  setTimeout(() => {
    btn.textContent = old;
    btn.classList.remove('ok');
  }, 1200);
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text?: string): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (cls) node.className = cls;
  if (text !== undefined) node.textContent = text;
  return node;
}

function button(text: string, cls: string): HTMLButtonElement {
  const b = el('button', cls, text);
  b.type = 'button';
  return b;
}
