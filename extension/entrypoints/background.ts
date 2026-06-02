import { browser } from 'wxt/browser'
import { defineBackground } from 'wxt/sandbox'

const DEFAULT_PORT = 8765
const POLL_INTERVAL = 5000
const ANIM_INTERVAL = 1000

let animTimer: ReturnType<typeof setInterval> | null = null
let animFrame = false
let prevActive = 0

export default defineBackground({
  main() {
    browser.runtime.onMessage.addListener((message: any, _sender: any, sendResponse: (response?: any) => void) => {
      if (message.type === 'DOWNLOAD_MODEL') {
        handleDownload(message.data, sendResponse)
        return true
      }
      if (message.type === 'DOWNLOAD_QUEUED') {
        handleQueuedDownload(message.data, sendResponse)
        return true
      }
      if (message.type === 'CHECK_HEALTH') {
        handleHealthCheck(message.port || DEFAULT_PORT, sendResponse)
        return true
      }
    })

    pollHealth()
    setInterval(pollHealth, POLL_INTERVAL)
  },
})

async function pollHealth() {
  const port = await getServerPort()
  try {
    const res = await fetch(`http://127.0.0.1:${port}/health`)
    const data = await res.json()
    const active = (data.active || 0) + (data.queued || 0)

    if (active > 0) {
      if (prevActive === 0) startAnim()
    } else {
      if (prevActive > 0) stopAnim('default')
    }
    prevActive = active
  } catch {
    if (prevActive > 0) stopAnim('err')
    else setIcon('err')
    prevActive = -1
  }
}

function startAnim() {
  stopAnim(null)
  animFrame = false
  animTimer = setInterval(() => {
    animFrame = !animFrame
    setIcon(animFrame ? '01' : 'default')
  }, ANIM_INTERVAL)
}

function stopAnim(fallback: string | null) {
  if (animTimer !== null) {
    clearInterval(animTimer)
    animTimer = null
  }
  if (fallback) setIcon(fallback)
}

function setIcon(name: string) {
  browser.action.setIcon({ path: { '128': `icons/${name}.png` } }).catch(() => {})
}

async function handleDownload(data: any, sendResponse: (r: any) => void) {
  try {
    const result = await sendDownloadRequest({
      modelVersionId: data.modelVersionId,
      fileId: data.fileId,
      modelName: data.modelName,
      fileSize: data.fileSize,
      previewImage: data.previewImage,
      priority: data.priority || 1,
    })
    sendResponse(result)
  } catch (err: any) {
    sendResponse({ error: err.message })
  }
}

async function handleQueuedDownload(data: any, sendResponse: (r: any) => void) {
  try {
    const result = await sendDownloadRequest({
      modelVersionId: data.modelVersionId,
      fileId: data.fileId,
      modelName: data.modelName,
      fileSize: data.fileSize,
      previewImage: data.previewImage,
      priority: data.priority || 3,
    })
    sendResponse(result)
  } catch (err: any) {
    sendResponse({ error: err.message })
  }
}

async function handleHealthCheck(port: number, sendResponse: (r: any) => void) {
  try {
    const res = await fetch(`http://127.0.0.1:${port}/health`)
    const data = await res.json()
    sendResponse(data)
  } catch {
    sendResponse({ status: 'error', error: 'Connection refused' })
  }
}

async function getServerPort(): Promise<number> {
  const result = await browser.storage.local.get('serverPort')
  return result.serverPort || DEFAULT_PORT
}

async function getApiKey(): Promise<string | undefined> {
  const result = await browser.storage.local.get('apiKey')
  return result.apiKey
}

async function sendDownloadRequest(request: {
  modelVersionId: number
  fileId: number
  modelType?: string
  baseModel?: string
  modelName?: string
  fileName?: string
  fileSize?: string
  previewImage?: string
  apiKey?: string
  priority?: number
}): Promise<any> {
  const port = await getServerPort()
  const savedKey = await getApiKey()

  if (!request.apiKey && savedKey) {
    request.apiKey = savedKey
  }

  const response = await fetch(`http://127.0.0.1:${port}/download`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  })

  if (!response.ok) {
    const errorData = await response.json().catch(() => ({}))
    throw new Error(errorData.error || `Server responded with ${response.status}`)
  }

  return await response.json()
}
