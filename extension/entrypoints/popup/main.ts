import { browser } from 'wxt/browser'

const DEFAULT_PORT = 8765

async function loadSettings() {
  const result = await browser.storage.local.get(['serverPort', 'apiKey'])
  const portInput = document.getElementById('server-port') as HTMLInputElement
  const keyInput = document.getElementById('api-key') as HTMLInputElement

  if (portInput && result.serverPort) portInput.value = String(result.serverPort)
  if (keyInput && result.apiKey) keyInput.value = result.apiKey
}

async function saveSettings() {
  const portInput = document.getElementById('server-port') as HTMLInputElement
  const keyInput = document.getElementById('api-key') as HTMLInputElement

  if (portInput) {
    await browser.storage.local.set({ serverPort: parseInt(portInput.value) || DEFAULT_PORT })
  }
  if (keyInput) {
    await browser.storage.local.set({ apiKey: keyInput.value })
  }
}

async function checkConnection() {
  const portInput = document.getElementById('server-port') as HTMLInputElement
  const port = parseInt(portInput?.value) || DEFAULT_PORT
  const badge = document.getElementById('status-badge')

  if (badge) {
    badge.textContent = '⏳ Проверка...'
    badge.className = ''
  }

  try {
    const response = await fetch(`http://127.0.0.1:${port}/health`)
    if (!response.ok) throw new Error('Not OK')

    const data = await response.json()
    if (badge) {
      badge.textContent = `✅ Connected (${data.active || 0} active)`
      badge.className = 'connected'
    }

    const activeEl = document.getElementById('active-count')
    const queuedEl = document.getElementById('queued-count')
    if (activeEl) activeEl.textContent = String(data.active || 0)
    if (queuedEl) queuedEl.textContent = String(data.queued || 0)
  } catch {
    if (badge) {
      badge.textContent = '❌ Сервер недоступен'
      badge.className = 'error'
    }
  }
}

document.addEventListener('DOMContentLoaded', async () => {
  await loadSettings()

  const checkBtn = document.getElementById('check-connection')
  checkBtn?.addEventListener('click', checkConnection)

  const toggleKeyBtn = document.getElementById('toggle-key-visibility')
  const keyInput = document.getElementById('api-key') as HTMLInputElement
  toggleKeyBtn?.addEventListener('click', () => {
    if (keyInput) {
      keyInput.type = keyInput.type === 'password' ? 'text' : 'password'
    }
  })

  const refreshQueueBtn = document.getElementById('refresh-queue')
  refreshQueueBtn?.addEventListener('click', checkConnection)

  const openLogsBtn = document.getElementById('open-logs')
  openLogsBtn?.addEventListener('click', () => {
    browser.tabs.create({ url: 'http://127.0.0.1:8765/logs' }).catch(() => {})
  })

  const openConfigBtn = document.getElementById('open-config')
  openConfigBtn?.addEventListener('click', () => {
    browser.tabs.create({ url: 'http://127.0.0.1:8765/config' }).catch(() => {})
  })

  const portInput = document.getElementById('server-port')
  portInput?.addEventListener('change', saveSettings)

  keyInput?.addEventListener('change', saveSettings)

  await checkConnection()
})

export {}
