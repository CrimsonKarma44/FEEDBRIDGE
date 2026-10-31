import { browser } from 'wxt/browser';

// The popup sets a per-tab badge with the feed count after a scan. This
// clears it when that tab navigates, so a stale count never lingers. It is
// the only background work the extension does.
export default defineBackground(() => {
  browser.tabs.onUpdated.addListener((tabId, info) => {
    if (info.status === 'loading') {
      void browser.action.setBadgeText({ tabId, text: '' });
    }
  });
});
