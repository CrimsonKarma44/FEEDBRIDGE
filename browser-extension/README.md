# FeedBridge browser extension

Chrome and Firefox extension (Manifest V3, built with [WXT](https://wxt.dev)). Click the toolbar icon to see the RSS, Atom and JSON feeds on the current page, then copy one, open it, or subscribe through the FeedBridge Telegram bot.

## How it behaves

- **Nothing runs until you click the icon.** There's no content script on every page. A click grants `activeTab` for that tab, and the popup runs one function (`utils/scan.ts`) that reads the page's `<link rel="alternate">` tags.
- **The backend is only called when needed:**
  - automatically on YouTube and GitHub, whose feeds aren't in the page tags
  - when you press **Scan with FeedBridge**
  - when you subscribe
- **Badge:** after a scan it shows that tab's feed count, and clears when the tab navigates. `entrypoints/background.ts` does nothing else.
- **Subscribe:** the popup asks `POST /v1/link-tokens` for a token and opens `t.me/<bot>?start=add_<token>` (or `?startgroup=` for a group). The bot asks the chat to confirm. If the server can't be reached, the popup shows `/addfeed <url>` to copy instead.

Permissions: `activeTab`, `scripting`, `storage`, and host access to `https://feedbridge.duckdns.org/*` only. A custom server set on the options page is requested at runtime.

## Install from a release

Download the latest `feedbridge-extension-<version>-chrome.zip` or `-firefox.zip` from [GitHub Releases](https://github.com/CrimsonKarma44/FEEDBRIDGE/releases). Each release page has install steps.

- **Chrome, Edge, Brave:** unzip it into a folder you'll keep, then `chrome://extensions` → Developer mode → **Load unpacked** → that folder.
- **Firefox:** `about:debugging#/runtime/this-firefox` → **Load Temporary Add-on** → the zip. It goes away when Firefox restarts unless the release includes a signed `.xpi`.

## Release

`.github/workflows/extension-release.yml` builds and publishes a release when a matching tag is pushed:

```bash
# 1. bump "version" in browser-extension/package.json (e.g. 0.2.0) and commit
# 2. tag that commit and push the tag
git tag extension-v0.2.0
git push origin extension-v0.2.0
```

The workflow checks that the tag matches `package.json`, runs `npm run check` and `npm test`, builds the Chrome, Firefox and source zips, writes `SHA256SUMS.txt`, and publishes the release with `RELEASE_NOTES.md` (`{{VERSION}}` is filled in). If the repository has the secrets `AMO_JWT_ISSUER` and `AMO_JWT_SECRET`, it also attaches a Mozilla-signed `-firefox-signed.xpi` that installs permanently in Firefox.

## Develop

```bash
npm install              # also runs `wxt prepare`
npm run dev              # Chrome with live reload
npm run dev:firefox
npm test                 # Vitest: scan.ts (jsdom), api.ts, links and settings
npm run check            # TypeScript
npm run build            # .output/chrome-mv3
npm run build:firefox    # .output/firefox-mv3
npm run zip              # store packages
```

Load a build by hand:
- **Chrome:** `chrome://extensions` → Developer mode → **Load unpacked** → `.output/chrome-mv3`.
- **Firefox:** `about:debugging#/runtime/this-firefox` → **Load Temporary Add-on** → `.output/firefox-mv3/manifest.json`.

Firefox builds need Firefox 142 or later, because they declare `data_collection_permissions` (`browsingActivity`). Check them with `npx web-ext lint --source-dir .output/firefox-mv3`.

## Layout

```
entrypoints/background.ts   clears the badge on navigation
entrypoints/popup/          scan, list, copy/open/subscribe
entrypoints/options/        server address and bot username
utils/scan.ts               injected <link> reader (must stay self-contained)
utils/api.ts                /v1/detect and /v1/link-tokens client
utils/telegram.ts           deep links and the /addfeed fallback
utils/settings.ts           storage.sync settings and validation
utils/feeds.ts              types, grouping, known platforms
public/icon/                icons cut from docs/images/feedbridge-logo.png
```
