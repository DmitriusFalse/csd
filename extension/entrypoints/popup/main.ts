const DEFAULT_PORT = 8765
let refreshTimer: ReturnType<typeof setInterval> | null = null
let configOpen = false
let helpOpen = false

function $<T = HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null
}

async function loadSettings() {
  const { serverPort, apiKey } = await chrome.storage.local.get(['serverPort', 'apiKey'])
  const portEl = document.getElementById('config-port') as HTMLInputElement
  if (portEl) portEl.value = String(serverPort || DEFAULT_PORT)
  const keyEl = document.getElementById('config-key') as HTMLInputElement
  if (keyEl) keyEl.value = apiKey || ''
}

async function saveSettings() {
  const el = document.getElementById('config-port') as HTMLInputElement
  if (el) await chrome.storage.local.set({ serverPort: parseInt(el.value) || DEFAULT_PORT })
  const el2 = document.getElementById('config-key') as HTMLInputElement
  if (el2) await chrome.storage.local.set({ apiKey: el2.value })
  closeConfig()
  refreshTasks()
}

async function savePort() {
  const el = $<HTMLInputElement>('server-port')
  if (el) await chrome.storage.local.set({ serverPort: parseInt(el.value) || DEFAULT_PORT })
}

async function saveKey() {
  const el = $<HTMLInputElement>('api-key')
  if (el) await chrome.storage.local.set({ apiKey: el.value })
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

function createTaskCard(task: any): HTMLDivElement {
  const card = document.createElement('div')
  card.className = 'task-card' + (task.status === 'completed' ? ' completed' : '') + (task.status === 'failed' ? ' failed' : '')

  const img = document.createElement('img')
  img.className = 'task-img'
  img.src = task.previewImage || ''
  img.alt = task.modelName || 'Model'
  img.onerror = () => { img.src = ''; img.style.display = 'none' }
  if (!task.previewImage) img.style.display = 'none'

  const info = document.createElement('div')
  info.className = 'task-info'

  const name = document.createElement('div')
  name.className = 'task-name'
  name.textContent = task.modelName || `Model #${task.modelVersionId}`

  const meta = document.createElement('div')
  meta.className = 'task-meta'

  if (task.status === 'downloading') {
    const downloaded = formatBytes(task.downloadedBytes || 0)
    const total = formatBytes(task.fileSizeBytes || 0)
    meta.textContent = `${downloaded} / ${total}`
    info.append(name, meta)

    const bar = document.createElement('div')
    bar.className = 'progress-bar'
    const fill = document.createElement('div')
    fill.className = 'progress-fill'
    const pct = Math.min(task.progress || 0, 100)
    fill.style.width = pct + '%'
    bar.appendChild(fill)
    info.appendChild(bar)

    const status = document.createElement('div')
    status.className = 'task-status'
    status.textContent = Math.round(pct) + '%'
    status.style.color = '#667eea'
    info.appendChild(status)
  } else if (task.status === 'queued' || task.status === 'paused') {
    meta.textContent = task.status === 'queued' ? 'В очереди' : 'На паузе'
    info.append(name, meta)
  } else if (task.status === 'completed') {
    meta.textContent = 'Готово'
    info.append(name, meta)
  } else if (task.status === 'failed') {
    meta.textContent = task.error || 'Ошибка'
    info.append(name, meta)
  }

  card.append(img, info)

  const actions = document.createElement('div')
  actions.className = 'task-actions'

  if (task.status === 'downloading' || task.status === 'queued' || task.status === 'paused') {
    const cancelBtn = document.createElement('button')
    cancelBtn.className = 'btn-icon'
    cancelBtn.textContent = '✕'
    cancelBtn.title = 'Cancel'
    cancelBtn.onclick = async () => {
      try {
        const port = parseInt($<HTMLInputElement>('server-port')?.value || String(DEFAULT_PORT))
        await fetch(`http://127.0.0.1:${port}/queue/${task.id}/cancel`, { method: 'POST' })
      } catch {}
    }
    actions.appendChild(cancelBtn)
  }

  card.appendChild(actions)
  return card
}

function renderTasks(data: any) {
  const activeList = $('active-list')
  const queuedList = $('queued-list')
  const historyList = $('history-list')
  const activeEmpty = $('active-empty')
  const queuedEmpty = $('queued-empty')
  const historyEmpty = $('history-empty')
  if (!activeList || !queuedList || !historyList) return

  activeList.innerHTML = ''
  queuedList.innerHTML = ''
  historyList.innerHTML = ''

  const active = data.active || []
  const queued = data.queued || []
  const history = data.history || []

  active.forEach((t: any) => activeList.appendChild(createTaskCard(t)))
  queued.forEach((t: any) => queuedList.appendChild(createTaskCard(t)))
  history.forEach((t: any) => historyList.appendChild(createTaskCard(t)))

  if (activeEmpty) activeEmpty.style.display = active.length ? 'none' : ''
  if (queuedEmpty) queuedEmpty.style.display = queued.length ? 'none' : ''
  if (historyEmpty) historyEmpty.style.display = history.length ? 'none' : ''
}

async function fetchTasks() {
  const port = parseInt($<HTMLInputElement>('server-port')?.value || String(DEFAULT_PORT))
  try {
    const res = await fetch(`http://127.0.0.1:${port}/tasks`, { signal: AbortSignal.timeout(5000) })
    if (!res.ok) throw new Error(String(res.status))
    renderTasks(await res.json())
    setStatus('connected')
  } catch {
    setStatus('disconnected')
    stopAutoRefresh()
  }
}

async function loadConfig() {
  const port = parseInt($<HTMLInputElement>('server-port')?.value || String(DEFAULT_PORT))
  const statusEl = $('cfg-status')
  try {
    const res = await fetch(`http://127.0.0.1:${port}/api/config`, { signal: AbortSignal.timeout(3000) })
    if (!res.ok) {
      if (statusEl) { statusEl.textContent = '❌ Сервер вернул ' + res.status; statusEl.className = 'cfg-status err' }
      return
    }
    const cfg = await res.json()
    setValue('cfg-root-path', cfg.root_path)
    setValue('cfg-max-concurrent', cfg.max_concurrent)
    setValue('cfg-retry-attempts', cfg.retry_attempts)
    setChecked('cfg-allow-nsfw', cfg.allow_nsfw)
    setChecked('cfg-sep-folder', cfg.separate_folder)
    setChecked('cfg-save-json', cfg.save_json)
    setValue('cfg-log-level', cfg.log_level)
    setValue('cfg-lm-url', cfg.lm_base_url || 'http://127.0.0.1:8188')
    setChecked('cfg-lora-enabled', cfg.lora_enabled)
    const whPort = cfg.webhook_url ? extractPort(cfg.webhook_url) : ''
    checkWebhook(whPort)
    if (statusEl) { statusEl.textContent = ''; statusEl.className = 'cfg-status' }
  } catch (e: any) {
    if (statusEl) { statusEl.textContent = '❌ ' + e.message; statusEl.className = 'cfg-status err' }
  }
}

function extractPort(url: string): string {
  try { return String(new URL(url).port) } catch { return '' }
}

function buildWebhookUrl(port: string): string {
  const p = parseInt(port)
  return p ? `http://127.0.0.1:${p}/api/lm/loras/scan?full_rebuild=false` : ''
}

async function checkWebhook(port: string) {
  const dot = $('cfg-webhook-status')
  if (!dot) return
  const p = parseInt(port)
  if (!p) { dot.className = 'wh-dot'; return }
  dot.className = 'wh-dot'
  try {
    const url = buildWebhookUrl(port)
    const res = await fetch(url, { signal: AbortSignal.timeout(3000) })
    dot.className = res.ok || res.status === 400 ? 'wh-dot ok' : 'wh-dot err'
  } catch { dot.className = 'wh-dot err' }
}

function setValue(id: string, v: any) {
  const el = $(id) as HTMLInputElement | HTMLSelectElement
  if (el) el.value = v ?? ''
}

function setChecked(id: string, v: any) {
  const el = $(id) as HTMLInputElement
  if (el) el.checked = !!v
}

async function saveConfig() {
  const lmBaseURL = getValue('cfg-lm-url') || 'http://127.0.0.1:8188'
  const whPort = extractPort(lmBaseURL)
  const cfg = {
    root_path: getValue('cfg-root-path'),
    max_concurrent: parseInt(getValue('cfg-max-concurrent')) || 2,
    retry_attempts: parseInt(getValue('cfg-retry-attempts')) || 3,
    retry_delay_seconds: 60,
    allow_nsfw: getChecked('cfg-allow-nsfw'),
    separate_folder: getChecked('cfg-sep-folder'),
    save_json: getChecked('cfg-save-json'),
    log_level: getValue('cfg-log-level'),
    lora_enabled: getChecked('cfg-lora-enabled'),
    lm_base_url: lmBaseURL,
    webhook_url: buildWebhookUrl(whPort),
  }
  const port = parseInt($<HTMLInputElement>('server-port')?.value || String(DEFAULT_PORT))
  const statusEl = $('cfg-status')
  try {
    const res = await fetch(`http://127.0.0.1:${port}/api/config`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
      signal: AbortSignal.timeout(5000),
    })
    const d = await res.json()
    if (d.status === 'saved') {
      if (statusEl) { statusEl.textContent = '✅ Сохранено'; statusEl.className = 'cfg-status ok' }
      setTimeout(() => toggleConfig(), 600)
    } else {
      if (statusEl) { statusEl.textContent = '❌ ' + (d.error || ''); statusEl.className = 'cfg-status err' }
    }
  } catch (e: any) {
    if (statusEl) { statusEl.textContent = '❌ ' + e.message; statusEl.className = 'cfg-status err' }
  }
}

function toggleConfig() {
  const panel = $('config-section')
  if (!panel) return
  configOpen = !configOpen
  panel.classList.toggle('open', configOpen)
  if (configOpen) { loadConfig(); closeHelp() }
}

function toggleHelp() {
  const panel = $('help-section')
  if (!panel) return
  helpOpen = !helpOpen
  panel.classList.toggle('open', helpOpen)
  if (helpOpen) closeConfig()
}

function closeConfig() {
  const panel = $('config-section')
  if (!panel) return
  configOpen = false
  panel.classList.remove('open')
}

function closeHelp() {
  const panel = $('help-section')
  if (!panel) return
  helpOpen = false
  panel.classList.remove('open')
}

function getValue(id: string): string {
  return ($(id) as HTMLInputElement)?.value ?? ''
}

function getChecked(id: string): boolean {
  return ($(id) as HTMLInputElement)?.checked ?? false
}

function setStatus(state: 'connected' | 'disconnected' | 'checking') {
  const dot = $<HTMLButtonElement>('status-indicator')
  if (!dot) return
  dot.className = 'status-dot ' + state
  if (state === 'connected') dot.title = 'Подключено'
  else if (state === 'disconnected') dot.title = 'Нет подключения — нажмите'
  else dot.title = 'Проверка...'
}

async function checkConnection() {
  const port = parseInt($<HTMLInputElement>('server-port')?.value || String(DEFAULT_PORT))
  setStatus('checking')
  try {
    const res = await fetch(`http://127.0.0.1:${port}/health`, { signal: AbortSignal.timeout(3000) })
    if (!res.ok) throw new Error(String(res.status))
    setStatus('connected')
    await Promise.all([fetchTasks(), configOpen ? loadConfig() : Promise.resolve()])
    startAutoRefresh()
  } catch {
    setStatus('disconnected')
    stopAutoRefresh()
  }
}

async function periodicCheck() {
  const port = parseInt($<HTMLInputElement>('server-port')?.value || String(DEFAULT_PORT))
  try {
    const res = await fetch(`http://127.0.0.1:${port}/health`, { signal: AbortSignal.timeout(3000) })
    if (!res.ok) throw new Error(String(res.status))
    setStatus('connected')
    await fetchTasks()
  } catch {
    setStatus('disconnected')
    stopAutoRefresh()
  }
}

function startAutoRefresh() {
  if (refreshTimer) return
  periodicCheck()
  refreshTimer = setInterval(periodicCheck, 5000)
}

function stopAutoRefresh() {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
}

async function init() {
  await loadSettings()

  const statusBtn = $('status-indicator')
  if (statusBtn) statusBtn.onclick = checkConnection

  $('toggle-key')!.onclick = () => {
    const el = $<HTMLInputElement>('api-key')
    if (el) el.type = el.type === 'password' ? 'text' : 'password'
  }

  const portEl = $<HTMLInputElement>('server-port')
  if (portEl) portEl.onchange = savePort

  const keyEl = $<HTMLInputElement>('api-key')
  if (keyEl) keyEl.oninput = saveKey

  $('open-logs')!.onclick = async () => {
    const port = parseInt(portEl?.value || String(DEFAULT_PORT))
    try {
      await fetch(`http://127.0.0.1:${port}/logs/open`, { method: 'POST', signal: AbortSignal.timeout(3000) })
    } catch {}
  }

  $('toggle-config')!.onclick = toggleConfig
  $('toggle-help')!.onclick = toggleHelp
  $('cfg-save')!.onclick = saveConfig

  const boosty = $('donate-boosty')
  if (boosty) boosty.onclick = (e) => { e.preventDefault(); chrome.tabs.create({ url: 'https://boosty.to/sir.geronis/donate' }) }
  const patreon = $('donate-patreon')
  if (patreon) patreon.onclick = (e) => { e.preventDefault(); chrome.tabs.create({ url: 'https://www.patreon.com/16134050/join' }) }
  const dBoosty = $('donate-footer-boosty')
  if (dBoosty) dBoosty.onclick = (e) => { e.preventDefault(); chrome.tabs.create({ url: 'https://boosty.to/sir.geronis/donate' }) }
  const dPatreon = $('donate-footer-patreon')
  if (dPatreon) dPatreon.onclick = (e) => { e.preventDefault(); chrome.tabs.create({ url: 'https://www.patreon.com/16134050/join' }) }

  await checkConnection()
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init)
} else {
  init()
}
