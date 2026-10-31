FeedBridge browser extension {{VERSION}} for Chrome and Firefox.

Click the toolbar icon on any page to see its RSS, Atom and JSON feeds, copy or open them, or subscribe through the [FeedBridge Telegram bot](https://t.me/feed_bridge_bot) in a couple of taps.

## Download

| File | For |
|---|---|
| `feedbridge-extension-{{VERSION}}-chrome.zip` | Chrome, Edge, Brave and other Chromium browsers |
| `feedbridge-extension-{{VERSION}}-firefox.zip` | Firefox 142 or later |
| `feedbridge-extension-{{VERSION}}-sources.zip` | Source code, as used to build both packages |
| `SHA256SUMS.txt` | Checksums for the files above |

## Install

The extension isn't in the Chrome Web Store or Firefox Add-ons yet, so it's installed by hand.

**Chrome, Edge, Brave**
1. Download `feedbridge-extension-{{VERSION}}-chrome.zip` and unzip it into a folder you'll keep. The browser loads it from there.
2. Open `chrome://extensions` (`edge://extensions` in Edge) and turn on **Developer mode**.
3. Click **Load unpacked** and choose the unzipped folder.
4. Pin FeedBridge from the puzzle-piece menu.

**Firefox**
1. Download `feedbridge-extension-{{VERSION}}-firefox.zip`.
2. Open `about:debugging#/runtime/this-firefox` and click **Load Temporary Add-on…**.
3. Choose the zip file.

Firefox removes temporary add-ons when it restarts, because this build isn't signed by Mozilla yet. The first time you use **Scan with FeedBridge** or **Subscribe**, Firefox asks to allow access to `feedbridge.duckdns.org`; choose **Allow**.

## What it does

- Reads the feed links on the current page when you click the icon. Nothing runs before that.
- On YouTube and GitHub, or when you press **Scan with FeedBridge**, asks the FeedBridge server to find feeds the page doesn't list.
- **Subscribe** opens Telegram with a single-use link; the bot asks you to confirm. You can subscribe your chat with the bot or a group (admins confirm in groups).
- If the server can't be reached, it shows the `/addfeed` command to paste into the bot instead.

Permissions: read the current tab when you click the icon, store your settings, and contact `feedbridge.duckdns.org`. The server only receives a page's address when you scan with FeedBridge, open the extension on YouTube or GitHub, or subscribe.
