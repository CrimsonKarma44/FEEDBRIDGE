import { defineConfig } from 'wxt';

// Default API. Users can point the extension elsewhere on the options page;
// that origin is then requested at runtime via optional_host_permissions.
export const DEFAULT_API_ORIGIN = 'https://feedbridge.duckdns.org';

export default defineConfig({
  // MV3 for Firefox too (WXT defaults Firefox to MV2).
  manifestVersion: 3,
  // Vite adds <link rel="modulepreload" crossorigin> for chunks shared by
  // the popup and options pages. Chrome ignores crossorigin preloads of an
  // extension's own files and logs a "cross-world extension resource
  // mismatch" warning; preloading local files gains nothing anyway.
  vite: () => ({
    build: { modulePreload: false },
  }),
  manifest: ({ browser }) => ({
    name: 'FeedBridge',
    description:
      'Find the RSS, Atom and JSON feeds on a page and subscribe through the FeedBridge Telegram bot.',
    // Nothing runs until the toolbar icon is clicked: activeTab grants the
    // current tab for that click, scripting reads its <link> tags once.
    permissions: ['activeTab', 'scripting', 'storage'],
    host_permissions: [`${DEFAULT_API_ORIGIN}/*`],
    optional_host_permissions: ['https://*/*', 'http://localhost/*', 'http://127.0.0.1/*'],
    ...(browser === 'firefox'
      ? {
          browser_specific_settings: {
            gecko: {
              id: 'feedbridge@crimsonkarma44.github.io',
              strict_min_version: '142.0',
              // Page addresses go to the FeedBridge server when the user
              // scans with FeedBridge or subscribes.
              data_collection_permissions: { required: ['browsingActivity'] },
            },
          },
        }
      : {}),
  }),
});
