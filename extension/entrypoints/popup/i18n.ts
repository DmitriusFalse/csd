export type Lang = 'en' | 'ru'

const STORAGE_KEY = 'csd_lang'

export function detectLang(): Lang {
  const browserLang = navigator.language || (navigator as any).userLanguage || ''
  if (browserLang.startsWith('ru') || browserLang.startsWith('uk') || browserLang.startsWith('be')) {
    return 'ru'
  }
  return 'en'
}

export async function loadLang(): Promise<Lang> {
  const stored = (await chrome.storage.local.get(STORAGE_KEY))[STORAGE_KEY]
  if (stored === 'en' || stored === 'ru') return stored
  return detectLang()
}

export async function saveLang(l: Lang) {
  await chrome.storage.local.set({ [STORAGE_KEY]: l })
}

type Dict = { [key: string]: string }

const en: Dict = {
  'title': 'Civitai Smart Downloader',
  'status-connected': 'Connected',
  'status-disconnected': 'No connection — click',
  'status-checking': 'Checking...',
  'btn-settings': 'Settings',
  'btn-help': 'Help',
  'help-header': '❓ Help',
  'help-desc': 'Browser extension for downloading models from Civitai via a local server.',
  'help-howto': 'How to use:',
  'help-step1': 'Run <code>csd-bridge.exe</code> (server)',
  'help-step2': 'Click the green dot 🟢 in the popup — check connection',
  'help-step3': 'Go to civitai.com or civitai.red',
  'help-step4': 'Click the purple "Download via CSD" button on the model page',
  'help-settings': 'Settings ⚙:',
  'help-port': '<b>Server port</b> — must match the port in <code>config.yaml</code>',
  'help-apikey': '<b>API key</b> — optional, can be set in <code>config.yaml</code>',
  'help-lm-path': '<b>Path from LM</b> — automatically resolves root path for each model type via Lora Manager',
  'help-lm-header': 'Lora Manager:',
  'help-lm-url': 'Set URL (default <code>http://127.0.0.1:8188</code>)',
  'help-lm-checkbox': 'Check <b>«Path from LM»</b> — path for LoRA/LyCORIS/Checkpoints is taken from LM',
  'help-lm-fallback': 'If LM does not respond or the type is not supported — default path is used',
  'help-logs': 'Server logs: 📄 button in settings',
  'help-config': 'Config: open <code>config.yaml</code> next to <code>csd-bridge.exe</code>',
  'help-donate': '❤ Support the project:',
  'config-header': '⚙ Settings',
  'config-port': 'Server port',
  'config-apikey': 'Civitai API key (optional)',
  'config-apikey-placeholder': 'Leave empty if set in config.yaml',
  'config-showhide': 'Show/hide',
  'config-savepath': 'Save path',
  'config-savepath-title': 'Default path. If "Path from LM" is enabled — root is taken from Lora Manager for each model type',
  'config-savepath-placeholder': 'D:/AI/Models',
  'config-lm-url': 'Lora Manager URL',
  'config-lm-url-placeholder': 'http://127.0.0.1:8188',
  'config-lm-check': 'Check',
  'config-lm-enabled': 'On',
  'config-lm-path': 'Path from LM',
  'config-max-concurrent': 'Max concurrent downloads',
  'config-retry': 'Retry attempts',
  'config-nsfw': 'Allow NSFW',
  'config-sep-folder': 'Separate folder for NSFW',
  'config-save-json': 'Save JSON metadata',
  'config-log-level': 'Log level',
  'config-open-logs': '📄 Open logs folder',
  'config-save-btn': '💾 Save',
  'config-saved': '✅ Saved',
  'section-active': '⬇ Active',
  'section-active-empty': 'No active downloads',
  'section-queued': '⏳ Queue',
  'section-queued-empty': 'Queue is empty',
  'section-history': '📋 History',
  'section-history-empty': 'History is empty',
  'task-pause': 'Pause',
  'task-resume': 'Resume',
  'task-cancel': 'Cancel',
  'task-queued': 'Queued',
  'task-paused': 'Paused',
  'task-completed': 'Done',
  'task-failed': 'Error',
  'lang-en': 'EN',
  'lang-ru': 'RU',
}

const ru: Dict = {
  'title': 'Civitai Smart Downloader',
  'status-connected': 'Подключено',
  'status-disconnected': 'Нет подключения — нажмите',
  'status-checking': 'Проверка...',
  'btn-settings': 'Настройки',
  'btn-help': 'Помощь',
  'help-header': '❓ Помощь',
  'help-desc': 'Расширение для скачивания моделей с Civitai через локальный сервер.',
  'help-howto': 'Как использовать:',
  'help-step1': 'Запусти <code>csd-bridge.exe</code> (сервер)',
  'help-step2': 'Нажми на зелёную точку 🟢 в попапе — проверить подключение',
  'help-step3': 'Зайди на civitai.com или civitai.red',
  'help-step4': 'Нажми фиолетовую кнопку "Download via CSD" на странице модели',
  'help-settings': 'Настройки ⚙:',
  'help-port': '<b>Порт сервера</b> — должен совпадать с портом в <code>config.yaml</code>',
  'help-apikey': '<b>API-ключ</b> — опционально, можно указать в <code>config.yaml</code>',
  'help-lm-path': '<b>Путь из LM</b> — автоматически определяет корень для каждого типа модели через Lora Manager',
  'help-lm-header': 'Lora Manager:',
  'help-lm-url': 'Укажи URL (по умолч. <code>http://127.0.0.1:8188</code>)',
  'help-lm-checkbox': 'Галка <b>«Путь из LM»</b> — путь для LoRA/LyCORIS/Checkpoints берётся из LM',
  'help-lm-fallback': 'Если LM не отвечает или тип не поддерживается — используется путь по умолчанию',
  'help-logs': 'Логи сервера: кнопка 📄 в настройках',
  'help-config': 'Config: открой <code>config.yaml</code> рядом с <code>csd-bridge.exe</code>',
  'help-donate': '❤ Поддержать проект:',
  'config-header': '⚙ Настройки',
  'config-port': 'Порт сервера',
  'config-apikey': 'API-ключ Civitai (опционально)',
  'config-apikey-placeholder': 'Оставь пустым, если указан в config.yaml',
  'config-showhide': 'Показать/скрыть',
  'config-savepath': 'Путь сохранения моделей',
  'config-savepath-title': 'Путь по умолчанию. Если включена "Путь из LM" — корень берётся из Lora Manager для каждого типа модели',
  'config-savepath-placeholder': 'D:/AI/Models',
  'config-lm-url': 'Lora Manager URL',
  'config-lm-url-placeholder': 'http://127.0.0.1:8188',
  'config-lm-check': 'Проверить подключение',
  'config-lm-enabled': 'Вкл',
  'config-lm-path': 'Путь из LM',
  'config-max-concurrent': 'Одновременных загрузок',
  'config-retry': 'Попыток при ошибке',
  'config-nsfw': 'Разрешить NSFW',
  'config-sep-folder': 'Отдельная папка для NSFW',
  'config-save-json': 'Сохранять JSON с метаданными',
  'config-log-level': 'Уровень логирования',
  'config-open-logs': '📄 Открыть папку с логами',
  'config-save-btn': '💾 Сохранить',
  'config-saved': '✅ Сохранено',
  'section-active': '⬇ Активные',
  'section-active-empty': 'Нет активных загрузок',
  'section-queued': '⏳ Очередь',
  'section-queued-empty': 'Очередь пуста',
  'section-history': '📋 История',
  'section-history-empty': 'История пуста',
  'task-pause': 'Пауза',
  'task-resume': 'Продолжить',
  'task-cancel': 'Cancel',
  'task-queued': 'В очереди',
  'task-paused': 'На паузе',
  'task-completed': 'Готово',
  'task-failed': 'Ошибка',
  'lang-en': 'EN',
  'lang-ru': 'RU',
}

const dicts: Record<Lang, Dict> = { en, ru }

let currentLang: Lang = 'en'

export function setLang(l: Lang) {
  currentLang = l
}

export function getLang(): Lang {
  return currentLang
}

export function t(key: string): string {
  return dicts[currentLang][key] ?? dicts['en'][key] ?? key
}

export function translateAll() {
  document.querySelectorAll('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n')
    if (!key) return
    const html = el.getAttribute('data-i18n-html') === 'true'
    if (html) {
      el.innerHTML = t(key)
    } else {
      el.textContent = t(key)
    }
  })

  document.querySelectorAll('[data-i18n-title]').forEach(el => {
    const key = el.getAttribute('data-i18n-title')
    if (!key) return
    (el as HTMLElement).title = t(key)
  })

  document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
    const key = el.getAttribute('data-i18n-placeholder')
    if (!key) return
    (el as HTMLInputElement).placeholder = t(key)
  })
}
