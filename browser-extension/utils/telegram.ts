export type SubscribeTarget = 'chat' | 'group';

/**
 * Deep link that opens the bot with "/start add_<token>". 'group' asks
 * Telegram to pick a group first (startgroup); the bot then asks an admin to
 * confirm there.
 */
export function deepLink(bot: string, token: string, target: SubscribeTarget): string {
  const param = target === 'group' ? 'startgroup' : 'start';
  return `https://t.me/${encodeURIComponent(bot)}?${param}=add_${encodeURIComponent(token)}`;
}

/** Plain link to the bot, for the copy-and-paste fallback. */
export function botLink(bot: string): string {
  return `https://t.me/${encodeURIComponent(bot)}`;
}

/** Command the user pastes into the bot when the backend can't make a token. */
export function addFeedCommand(feedUrl: string): string {
  return `/addfeed ${feedUrl}`;
}
