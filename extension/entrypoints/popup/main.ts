import { loadLang, saveLang, setLang, getLang, translateAll, t, type Lang } from './i18n.js'

const DEFAULT_PORT = 8765
let refreshTimer: ReturnType<typeof setInterval> | null = null
let configOpen = false
let helpOpen = false
let historyOpen = false

function $<T = HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null
}

async function loadSettings() {
  const { serverPort, apiKey } = await chrome.storage.local.get(['serverPort', 'apiKey'])
  const portEl = document.getElementById('server-port') as HTMLInputElement
  if (portEl) portEl.value = String(serverPort || DEFAULT_PORT)
  const keyEl = document.getElementById('api-key') as HTMLInputElement
  if (keyEl) keyEl.value = apiKey || ''
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

function formatDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  const locale = getLang() === 'ru' ? 'ru-RU' : 'en-US'
  return d.toLocaleDateString(locale, { day: 'numeric', month: 'short', year: 'numeric' })
}

function createTaskCard(task: any, index?: number): HTMLDivElement {
  const card = document.createElement('div')
  card.className = 'task-card' + (task.status === 'completed' ? ' completed' : '') + (task.status === 'failed' ? ' failed' : '')
  card.dataset.taskId = task.id

  const contents: HTMLElement[] = []

  if (index !== undefined) {
    const num = document.createElement('span')
    num.className = 'task-number'
    num.textContent = String(index)
    contents.push(num)
  }

  const img = document.createElement('img')
  img.className = 'task-img'
  img.src = task.previewImage || ''
  img.alt = task.modelName || 'Model'
  img.onerror = () => { img.src = ''; img.style.display = 'none' }
  if (!task.previewImage) img.style.display = 'none'
  contents.push(img)

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
    meta.textContent = task.status === 'queued' ? t('task-queued') : t('task-paused')
    info.append(name, meta)
  } else if (task.status === 'completed') {
    const dateStr = formatDate(task.completedAt)
    meta.textContent = t('task-completed') + (dateStr ? ` · ${dateStr}` : '')
    info.append(name, meta)
  } else if (task.status === 'failed') {
    meta.textContent = task.error || t('task-failed')
    info.append(name, meta)
  }

  contents.push(info)

  card.append(...contents)

  const actions = document.createElement('div')
  actions.className = 'task-actions'

  if (task.status === 'downloading' || task.status === 'paused' || task.status === 'queued') {
    const port = () => parseInt($<HTMLInputElement>('server-port')?.value || String(DEFAULT_PORT))

    if (task.status === 'downloading') {
      const pauseBtn = document.createElement('button')
      pauseBtn.className = 'btn-icon'
      pauseBtn.textContent = '⏸'
      pauseBtn.title = t('task-pause')
      pauseBtn.onclick = async () => {
        try { await fetch(`http://127.0.0.1:${port()}/queue/${task.id}/pause`, { method: 'POST' }) } catch {}
      }
      actions.appendChild(pauseBtn)
    }

    if (task.status === 'paused') {
      const resumeBtn = document.createElement('button')
      resumeBtn.className = 'btn-icon'
      resumeBtn.textContent = '▶'
      resumeBtn.title = t('task-resume')
      resumeBtn.onclick = async () => {
        try { await fetch(`http://127.0.0.1:${port()}/queue/${task.id}/resume`, { method: 'POST' }) } catch {}
      }
      actions.appendChild(resumeBtn)
    }

    const cancelBtn = document.createElement('button')
    cancelBtn.className = 'btn-icon'
    cancelBtn.textContent = '✕'
    cancelBtn.title = t('task-cancel')
    cancelBtn.onclick = async () => {
      try { await fetch(`http://127.0.0.1:${port()}/queue/${task.id}/cancel`, { method: 'POST' }) } catch {}
    }
    actions.appendChild(cancelBtn)
  }

  card.appendChild(actions)
  return card
}

function updateTaskCard(card: HTMLElement, task: any) {
  const info = card.querySelector('.task-info')!
  const meta = info.querySelector('.task-meta') as HTMLElement
  const oldStatus = card.className.includes('completed') ? 'completed' : card.className.includes('failed') ? 'failed' : ''

  card.className = 'task-card' + (task.status === 'completed' ? ' completed' : '') + (task.status === 'failed' ? ' failed' : '')

  if (task.status === 'downloading') {
    const bar = info.querySelector('.progress-bar') as HTMLElement
    const fill = info.querySelector('.progress-fill') as HTMLElement
    const statusEl = info.querySelector('.task-status') as HTMLElement
    const downloaded = formatBytes(task.downloadedBytes || 0)
    const total = formatBytes(task.fileSizeBytes || 0)
    const pct = Math.min(task.progress || 0, 100)

    if (bar && fill && statusEl) {
      meta.textContent = `${downloaded} / ${total}`
      fill.style.width = pct + '%'
      statusEl.textContent = Math.round(pct) + '%'
    } else {
      // Transitioned from another status: rebuild info section
      info.innerHTML = ''
      const name = document.createElement('div')
      name.className = 'task-name'
      name.textContent = task.modelName || `Model #${task.modelVersionId}`
      meta.textContent = `${downloaded} / ${total}`
      info.append(name, meta)
      const newBar = document.createElement('div')
      newBar.className = 'progress-bar'
      const newFill = document.createElement('div')
      newFill.className = 'progress-fill'
      newFill.style.width = pct + '%'
      newBar.appendChild(newFill)
      info.appendChild(newBar)
      const newStatus = document.createElement('div')
      newStatus.className = 'task-status'
      newStatus.textContent = Math.round(pct) + '%'
      newStatus.style.color = '#667eea'
      info.appendChild(newStatus)
    }
  } else {
    // Not downloading — just update meta text
    if (task.status === 'queued') meta.textContent = t('task-queued')
    else if (task.status === 'paused') meta.textContent = t('task-paused')
    else if (task.status === 'completed') meta.textContent = t('task-completed')
    else if (task.status === 'failed') meta.textContent = task.error || t('task-failed')

    // Remove progress elements if they exist (transitioned from downloading)
    const bar = info.querySelector('.progress-bar')
    const statusEl = info.querySelector('.task-status')
    if (bar) bar.remove()
    if (statusEl) statusEl.remove()
  }

  // Rebuild action buttons
  const actions = card.querySelector('.task-actions') as HTMLElement
  if (actions) actions.remove()
  const newActions = document.createElement('div')
  newActions.className = 'task-actions'
  if (task.status === 'downloading' || task.status === 'paused' || task.status === 'queued') {
    const port = () => parseInt($<HTMLInputElement>('server-port')?.value || String(DEFAULT_PORT))
    if (task.status === 'downloading') {
      const btn = document.createElement('button')
      btn.className = 'btn-icon'
      btn.textContent = '⏸'
      btn.title = t('task-pause')
      btn.onclick = async () => { try { await fetch(`http://127.0.0.1:${port()}/queue/${task.id}/pause`, { method: 'POST' }) } catch {} }
      newActions.appendChild(btn)
    }
    if (task.status === 'paused') {
      const btn = document.createElement('button')
      btn.className = 'btn-icon'
      btn.textContent = '▶'
      btn.title = t('task-resume')
      btn.onclick = async () => { try { await fetch(`http://127.0.0.1:${port()}/queue/${task.id}/resume`, { method: 'POST' }) } catch {} }
      newActions.appendChild(btn)
    }
    const cancelBtn = document.createElement('button')
    cancelBtn.className = 'btn-icon'
    cancelBtn.textContent = '✕'
    cancelBtn.title = t('task-cancel')
    cancelBtn.onclick = async () => { try { await fetch(`http://127.0.0.1:${port()}/queue/${task.id}/cancel`, { method: 'POST' }) } catch {} }
    newActions.appendChild(cancelBtn)
  }
  card.appendChild(newActions)
}

function renderTasks(data: any) {
  const containers: Record<string, HTMLElement | null> = {
    active: $('active-list'),
    queued: $('queued-list'),
    history: $('history-list'),
  }
  const empties: Record<string, HTMLElement | null> = {
    active: $('active-empty'),
    queued: $('queued-empty'),
    history: $('history-empty'),
  }
  if (!containers.active || !containers.queued || !containers.history) return

  const seen = new Set<string>()

  for (const section of ['active', 'queued', 'history'] as const) {
    let tasks = data[section] || []
    const container = containers[section]!

    // History: oldest first with numbering
    if (section === 'history') {
      tasks = [...tasks].reverse()
    }

    for (let i = 0; i < tasks.length; i++) {
      const task = tasks[i]
      seen.add(task.id)
      let card = container.querySelector(`[data-task-id="${task.id}"]`) as HTMLElement
      if (card) {
        updateTaskCard(card, task)
      } else {
        // Check if card exists in another section (status change moved it)
        const moved = document.querySelector(`[data-task-id="${task.id}"]`) as HTMLElement
        if (moved) {
          moved.remove()
          container.appendChild(moved)
          updateTaskCard(moved, task)
        } else {
          const idx = section === 'history' ? i + 1 : undefined
          card = createTaskCard(task, idx)
          container.appendChild(card)
        }
      }
    }

    // Remove stale cards
    let child = container.firstElementChild
    while (child) {
      const next = child.nextElementSibling
      const id = (child as HTMLElement).dataset.taskId
      if (id && !seen.has(id)) {
        child.remove()
      }
      child = next
    }

    if (empties[section]) {
      empties[section]!.style.display = tasks.length ? 'none' : ''
    }
  }
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
      if (statusEl) { statusEl.textContent = '❌ Server returned ' + res.status; statusEl.className = 'cfg-status err' }
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
    setChecked('cfg-lm-enabled', cfg.lora_enabled)
    setChecked('cfg-use-lm-path', cfg.use_lm_path)
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
    lora_enabled: getChecked('cfg-lm-enabled'),
    use_lm_path: getChecked('cfg-use-lm-path'),
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
      if (statusEl) { statusEl.textContent = t('config-saved'); statusEl.className = 'cfg-status ok' }
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

function toggleHistory() {
  const body = $('history-body')
  const header = $('history-toggle')
  if (!body || !header) return
  historyOpen = !historyOpen
  body.classList.toggle('open', historyOpen)
  header.classList.toggle('open', historyOpen)
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
  if (state === 'connected') dot.title = t('status-connected')
  else if (state === 'disconnected') dot.title = t('status-disconnected')
  else dot.title = t('status-checking')
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
  refreshTimer = setInterval(periodicCheck, 1000)
}

function stopAutoRefresh() {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
}

async function setAppLang(l: Lang) {
  setLang(l)
  await saveLang(l)
  translateAll()

  const enBtn = $('lang-en')
  const ruBtn = $('lang-ru')
  if (enBtn) enBtn.classList.toggle('active', l === 'en')
  if (ruBtn) ruBtn.classList.toggle('active', l === 'ru')

  setStatusFromCurrent()
  await fetchTasks()
}

function setStatusFromCurrent() {
  const dot = $<HTMLButtonElement>('status-indicator')
  if (!dot) return
  if (dot.classList.contains('connected')) setStatus('connected')
  else if (dot.classList.contains('disconnected')) setStatus('disconnected')
  else setStatus('checking')
}

async function init() {
  const savedLang = await loadLang()
  await setAppLang(savedLang)

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
  $('history-toggle')!.onclick = toggleHistory

  const boosty = $('donate-boosty')
  if (boosty) boosty.onclick = (e) => { e.preventDefault(); chrome.tabs.create({ url: 'https://boosty.to/sir.geronis/donate' }) }
  const patreon = $('donate-patreon')
  if (patreon) patreon.onclick = (e) => { e.preventDefault(); chrome.tabs.create({ url: 'https://www.patreon.com/16134050/join' }) }
  const dBoosty = $('donate-footer-boosty')
  if (dBoosty) dBoosty.onclick = (e) => { e.preventDefault(); chrome.tabs.create({ url: 'https://boosty.to/sir.geronis/donate' }) }
  const dPatreon = $('donate-footer-patreon')
  if (dPatreon) dPatreon.onclick = (e) => { e.preventDefault(); chrome.tabs.create({ url: 'https://www.patreon.com/16134050/join' }) }

  const langEnBtn = $('lang-en')
  const langRuBtn = $('lang-ru')
  if (langEnBtn) langEnBtn.onclick = () => setAppLang('en')
  if (langRuBtn) langRuBtn.onclick = () => setAppLang('ru')

  await checkConnection()
}

async function savePort() {
  const el = $<HTMLInputElement>('server-port')
  if (el) await chrome.storage.local.set({ serverPort: parseInt(el.value) || DEFAULT_PORT })
}

async function saveKey() {
  const el = $<HTMLInputElement>('api-key')
  if (el) await chrome.storage.local.set({ apiKey: el.value })
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init)
} else {
  init()
}
