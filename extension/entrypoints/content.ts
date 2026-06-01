import { browser } from 'wxt/browser'
import { defineContentScript } from 'wxt/sandbox'

export default defineContentScript({
  matches: ['*://civitai.com/*', '*://civitai.red/*'],
  runAt: 'document_idle',
  main() {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', init)
    } else {
      init()
    }
  },
})

interface PageData {
  modelVersionId: string | null
  fileId: string | null
  modelName: string
  fileSize: string
}

function extractPageData(): PageData | null {
  if (typeof window === 'undefined' || !window.location) return null

  const urlParams = new URLSearchParams(window.location.search)
  const modelVersionId = urlParams.get('modelVersionId')

  const downloadButton = document.querySelector<HTMLAnchorElement>('a[href*="/api/download/models/"]')
  if (!downloadButton) return null

  const href = downloadButton.getAttribute('href') || ''
  const queryString = href.split('?')[1] || ''
  const hrefParams = new URLSearchParams(queryString)
  const fileId = hrefParams.get('fileId')

  const modelName =
    document.querySelector<HTMLHeadingElement>('h1')?.textContent?.trim() ||
    document.title.replace(' - Civitai', '').trim() ||
    'Unknown Model'

  const fileSize = downloadButton.textContent?.trim() || ''

  return { modelVersionId, fileId, modelName, fileSize }
}

function injectDownloadButton(data: PageData) {
  if (!data.modelVersionId || !data.fileId) return

  const container = document.querySelector('a[href*="/api/download/models/"]')?.parentElement
  if (!container) return

  const existing = document.querySelector<HTMLButtonElement>('.csd-download-btn')
  if (existing) existing.remove()

  const btn = document.createElement('button')
  btn.className = 'csd-download-btn'
  btn.textContent = '⬇ Скачать в Lora Manager'
  btn.style.cssText = [
    'display: inline-flex',
    'align-items: center',
    'gap: 6px',
    'padding: 8px 16px',
    'margin: 4px',
    'border: none',
    'border-radius: 8px',
    'background: linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    'color: white',
    'font-size: 14px',
    'font-weight: 600',
    'cursor: pointer',
    'transition: opacity 0.2s',
  ].join(';')

  btn.onmouseenter = () => (btn.style.opacity = '0.9')
  btn.onmouseleave = () => (btn.style.opacity = '1')

  btn.addEventListener('click', async () => {
    btn.textContent = '⏳ Отправка...'
    btn.disabled = true

    try {
      const result = await browser.runtime.sendMessage({
        type: 'DOWNLOAD_MODEL',
        data: {
          modelVersionId: parseInt(data.modelVersionId),
          fileId: parseInt(data.fileId),
          modelName: data.modelName,
          fileSize: data.fileSize,
        },
      })

      if (result?.id) {
        showToast('✅ Задача отправлена: ' + data.modelName, 'success')
        btn.textContent = '✅ Отправлено'
        setTimeout(() => {
          btn.textContent = '⬇ Скачать в Lora Manager'
        }, 5000)
      } else if (result?.code) {
        showToast('❌ ' + getErrorMessage(result.code, result.error), 'error')
        btn.textContent = '⬇ Скачать в Lora Manager'
      } else {
        showToast('❌ Ошибка: ' + (result?.error || 'Неизвестная ошибка'), 'error')
        btn.textContent = '⬇ Скачать в Lora Manager'
      }
    } catch (err: any) {
      showToast('❌ Ошибка связи с сервером: ' + err.message, 'error')
      btn.textContent = '⬇ Скачать в Lora Manager'
    }

    btn.disabled = false
  })

  container.appendChild(btn)
}

function getErrorMessage(code: string, fallback: string): string {
  const messages: Record<string, string> = {
    UNAUTHORIZED: 'Неверный API-ключ. Проверьте ключ в настройках расширения.',
    FORBIDDEN: 'Нет доступа к модели. Возможно, модель приватная.',
    NOT_FOUND: 'Модель не найдена. Возможно, она была удалена.',
    RATE_LIMITED: 'Civitai ограничил запросы. Попробуйте позже.',
    CLOUDFLARE: 'Cloudflare защита. Повторите попытку через минуту.',
    NETWORK: 'Сервер недоступен. Запущен ли Civitai Smart Downloader?',
    INVALID_REQUEST: 'Некорректный запрос. Обновите расширение.',
    SERVER_ERROR: 'Ошибка сервера. Проверьте логи приложения.',
  }
  return messages[code] || fallback || 'Неизвестная ошибка'
}

function showToast(message: string, type: 'success' | 'error' | 'warning') {
  const toast = document.createElement('div')
  toast.textContent = message
  toast.style.cssText = [
    'position: fixed',
    'bottom: 24px',
    'right: 24px',
    'padding: 12px 20px',
    'border-radius: 8px',
    'color: white',
    'font-size: 14px',
    'font-weight: 500',
    'z-index: 99999',
    'box-shadow: 0 4px 12px rgba(0,0,0,0.2)',
    'animation: slideIn 0.3s ease',
    'max-width: 400px',
    'word-wrap: break-word',
    type === 'success' ? 'background: #10b981' : '',
    type === 'error' ? 'background: #ef4444' : '',
    type === 'warning' ? 'background: #f59e0b' : '',
  ].join(';')

  const style = document.createElement('style')
  style.textContent = [
    '@keyframes slideIn {',
    '  from { transform: translateX(100%); opacity: 0; }',
    '  to { transform: translateX(0); opacity: 1; }',
    '}',
  ].join('\n')

  document.head.appendChild(style)
  document.body.appendChild(toast)

  setTimeout(() => {
    toast.style.transition = 'opacity 0.3s'
    toast.style.opacity = '0'
    setTimeout(() => toast.remove(), 300)
  }, 6000)
}

function init() {
  if (typeof window === 'undefined' || !window.location) return

  const data = extractPageData()
  if (data && data.modelVersionId && data.fileId) {
    injectDownloadButton(data)
  }
}
