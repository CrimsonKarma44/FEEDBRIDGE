import { browser } from 'wxt/browser';
import '~/assets/theme.css';
import './style.css';
import {
  DEFAULT_SETTINGS,
  loadSettings,
  originPattern,
  parseApiOrigin,
  parseBotUsername,
  saveSettings,
} from '~/utils/settings';

const form = document.getElementById('form') as HTMLFormElement;
const api = document.getElementById('api') as HTMLInputElement;
const bot = document.getElementById('bot') as HTMLInputElement;
const error = document.getElementById('error')!;
const saved = document.getElementById('saved')!;

void loadSettings().then((s) => {
  api.value = s.apiOrigin;
  bot.value = s.botUsername;
});

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  error.hidden = true;
  saved.textContent = '';

  let apiOrigin: string;
  let botUsername: string;
  try {
    apiOrigin = parseApiOrigin(api.value);
    botUsername = parseBotUsername(bot.value);
  } catch (err) {
    showError((err as Error).message);
    return;
  }

  // A custom server needs host access. Ask now, during the click, before
  // any other await (Firefox only prompts during a user gesture).
  if (apiOrigin !== DEFAULT_SETTINGS.apiOrigin) {
    const granted = await browser.permissions
      .request({ origins: [originPattern(apiOrigin)] })
      .catch(() => false);
    if (!granted) {
      showError('FeedBridge needs permission to reach that server. Save again and choose Allow.');
      return;
    }
  }

  await saveSettings({ apiOrigin, botUsername });
  api.value = apiOrigin;
  bot.value = botUsername;
  saved.textContent = 'Saved';
});

document.getElementById('reset')!.addEventListener('click', async () => {
  await saveSettings(DEFAULT_SETTINGS);
  api.value = DEFAULT_SETTINGS.apiOrigin;
  bot.value = DEFAULT_SETTINGS.botUsername;
  error.hidden = true;
  saved.textContent = 'Defaults restored';
});

function showError(msg: string) {
  error.textContent = msg;
  error.hidden = false;
}
