import { browser } from 'wxt/browser'

const DEFAULT_PORT = 8765

interface DownloadRequest {
  modelVersionId: number
  fileId: number
  modelType?: string
  baseModel?: string
  modelName?: string
  fileName?: string
  fileSize?: string
  apiKey?: string
  priority?: number
}

async function getServerPort(): Promise<number> {
  const result = await browser.storage.local.get('serverPort')
  return result.serverPort || DEFAULT_PORT
}

async function getApiKey(): Promise<string | undefined> {
  const result = await browser.storage.local.get('apiKey')
  return result.apiKey
}

async function sendDownloadRequest(request: DownloadRequest): Promise<any> {
  const port = await getServerPort()
  const savedKey = await getApiKey()

  if (!request.apiKey && savedKey) {
    request.apiKey = savedKey
  }

  try {
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
  } catch (err: any) {
    if (err.message.includes('fetch')) {
      throw new Error('Сервер недоступен. Запущен ли Civitai Smart Downloader?')
    }
    throw err
  }
}

browser.runtime.onMessage.addListener((message: any, _sender: any, sendResponse: (response?: any) => void) => {
  if (message.type === 'DOWNLOAD_MODEL') {
    const { data } = message

    sendDownloadRequest({
      modelVersionId: data.modelVersionId,
      fileId: data.fileId,
      modelName: data.modelName,
      fileSize: data.fileSize,
      priority: data.priority || 1,
    })
      .then((result) => sendResponse(result))
      .catch((err) => sendResponse({ error: err.message }))

    return true
  }

  if (message.type === 'DOWNLOAD_QUEUED') {
    const { data } = message

    sendDownloadRequest({
      modelVersionId: data.modelVersionId,
      fileId: data.fileId,
      modelName: data.modelName,
      fileSize: data.fileSize,
      priority: data.priority || 3,
    })
      .then((result) => sendResponse(result))
      .catch((err) => sendResponse({ error: err.message }))

    return true
  }

  if (message.type === 'CHECK_HEALTH') {
    const port = message.port || DEFAULT_PORT
    fetch(`http://127.0.0.1:${port}/health`)
      .then((res) => res.json())
      .then((data) => sendResponse(data))
      .catch(() => sendResponse({ status: 'error', error: 'Connection refused' }))

    return true
  }
})

export default {}
