import { defineContentScript } from 'wxt/sandbox'

export default defineContentScript({
  matches: ['*://civitai.com/*', '*://civitai.red/*'],
  runAt: 'document_idle',
  main() {
    tryInject()
    observePageChanges()
  },
})

function findDownloadLink(): HTMLAnchorElement | null {
  return document.querySelector<HTMLAnchorElement>('a[href*="/api/download/models/"]')
}

function extractModelType(): string {
  const pageText = document.body.innerText
  const firstWords = pageText.substring(0, 300)

  const typeMatch = firstWords.match(/(Checkpoint|LoRA|Textual\s*Inversion|Embedding|Hypernetwork|ControlNet|Poses|Motion\s*Module|VAE|Upscaler|Wildcards|Workflows|Other)\b/i)
  if (typeMatch) {
    const t = typeMatch[1].toLowerCase().replace(/\s+/g, '')
    if (t === 'checkpoint') return 'Checkpoint'
    if (t === 'lora') return 'LoRA'
    if (t === 'textualinversion') return 'TextualInversion'
    if (t === 'embedding') return 'Embedding'
    if (t === 'hypernetwork') return 'Hypernetwork'
    if (t === 'controlnet') return 'ControlNet'
  }

  const typeSelectors = [
    'a[href*="type="]',
    '[class*="tag" i]',
    '[class*="badge" i]',
    '[class*="type" i]',
  ]
  for (const sel of typeSelectors) {
    const el = document.querySelector<HTMLElement>(sel)
    if (el) {
      const text = el.textContent?.trim() || ''
      const lower = text.toLowerCase()
      if (lower.includes('checkpoint')) return 'Checkpoint'
      if (lower.includes('lora')) return 'LoRA'
      if (lower.includes('textual') || lower.includes('embedding')) return 'TextualInversion'
      if (lower.includes('hypernetwork')) return 'Hypernetwork'
    }
  }

  const downloadLink = findDownloadLink()
  if (downloadLink) {
    const href = downloadLink.getAttribute('href') || ''
    const typeParam = new URLSearchParams(href.split('?')[1] || '').get('type')
    if (typeParam) {
      const lower = typeParam.toLowerCase()
      if (lower.includes('lora')) return 'LoRA'
      if (lower.includes('checkpoint')) return 'Checkpoint'
      if (lower.includes('embedding')) return 'TextualInversion'
    }
  }

  return 'LORA'
}

function extractModelId(): string | null {
  const match = window.location.pathname.match(/\/models\/(\d+)/)
  return match ? match[1] : null
}

function extractPageData() {
  const urlParams = new URLSearchParams(window.location.search)
  const modelVersionId = urlParams.get('modelVersionId')
  const downloadLink = findDownloadLink()
  if (!downloadLink || !modelVersionId) return null

  const href = downloadLink.getAttribute('href') || ''
  const queryString = href.split('?')[1] || ''
  const fileId = new URLSearchParams(queryString).get('fileId')

  const modelName =
    document.querySelector<HTMLHeadingElement>('h1')?.textContent?.trim() ||
    document.title.replace(/ - Civitai/i, '').trim() ||
    'Unknown Model'

  const fileSize = downloadLink.textContent?.trim() || ''

  const previewImage =
    document.querySelector<HTMLMetaElement>('meta[property="og:image"]')?.content ||
    document.querySelector<HTMLImageElement>('[class*="carousel"] img, [class*="ResourceImage"] img, [class*="preview"] img')?.src ||
    ''

  return { modelVersionId, fileId, modelName, fileSize, previewImage, modelId: extractModelId(), modelType: extractModelType() }
}

function injectButton(data: NonNullable<ReturnType<typeof extractPageData>>) {
  if (!data.modelVersionId || !data.fileId) return

  const link = findDownloadLink()
  if (!link) return
  const container = link.parentElement
  if (!container) return

  if (document.querySelector('.csd-btn')) return

  const btn = document.createElement('a')
  btn.className = 'csd-btn'
  btn.title = 'Download with Civitai Smart Downloader'
  btn.style.cssText = [
    'display:inline-flex',
    'align-items:center',
    'justify-content:center',
    'gap:8px',
    'padding:0 16px',
    'width:100%',
    'height:36px',
    'border:1px solid rgba(150,120,200,0.2)',
    'border-radius:8px',
    'background:rgba(120,80,180,0.1)',
    'color:rgb(160,130,200)',
    'font-size:14px',
    'font-weight:600',
    'cursor:pointer',
    'text-decoration:none',
    'white-space:nowrap',
    'transition:background 0.15s',
    'user-select:none',
    'margin:4px 0',
    'box-sizing:border-box',
  ].join(';')

  btn.addEventListener('mouseenter', () => {
    btn.style.background = 'rgba(120,80,180,0.2)'
  })
  btn.addEventListener('mouseleave', () => {
    btn.style.background = 'rgba(120,80,180,0.1)'
  })

  const inner = document.createElement('span')
  inner.style.cssText = 'display:inline-flex;align-items:center;gap:8px;'

  const iconSvg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
  iconSvg.setAttribute('width', '20')
  iconSvg.setAttribute('height', '20')
  iconSvg.setAttribute('viewBox', '0 0 24 24')
  iconSvg.setAttribute('fill', 'none')
  iconSvg.setAttribute('stroke', 'currentColor')
  iconSvg.setAttribute('stroke-width', '2')
  iconSvg.setAttribute('stroke-linecap', 'round')
  iconSvg.setAttribute('stroke-linejoin', 'round')
  iconSvg.innerHTML = '<path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2"></path><path d="M7 11l5 5l5 -5"></path><path d="M12 4l0 12"></path>'

  const textSpan = document.createElement('span')
  textSpan.textContent = 'Download via CSD'

  inner.append(iconSvg, textSpan)
  btn.appendChild(inner)

  container.appendChild(btn)

  // Check if already downloaded
  const modelIdVal = data.modelId || ''
  chrome.runtime.sendMessage({
    type: 'CHECK_DOWNLOADED',
    data: { name: data.modelName, type: data.modelType, modelId: modelIdVal },
  }).then((result: any) => {
    console.log('[CSD] check-downloaded response:', JSON.stringify(result))
    if (result?.downloaded) {
      textSpan.textContent = '✓ Downloaded'
      iconSvg.innerHTML = '<path d="M20 6L9 17l-5-5"></path>'
      btn.style.border = '1px solid rgba(16,185,129,0.3)'
      btn.style.background = 'rgba(16,185,129,0.1)'
      btn.style.color = 'rgb(16,185,129)'
      btn.title = 'Already downloaded'
      btn.addEventListener('mouseenter', () => {
        btn.style.background = 'rgba(16,185,129,0.2)'
      })
      btn.addEventListener('mouseleave', () => {
        btn.style.background = 'rgba(16,185,129,0.1)'
      })
    } else {
      console.log('[CSD] model not found via LM, server response:', result)
    }
  }).catch((err: any) => {
    console.log('[CSD] check-downloaded error:', err?.message || err)
  })

  btn.addEventListener('click', async (e) => {
    e.preventDefault()
    e.stopPropagation()

    if (textSpan.textContent === '✓ Downloaded') {
      showToast('Эта модель уже скачана', 'success')
      return
    }

    btn.style.pointerEvents = 'none'
    textSpan.textContent = '...'

    try {
      const result = await chrome.runtime.sendMessage({
        type: 'DOWNLOAD_MODEL',
        data: {
          modelVersionId: parseInt(data.modelVersionId),
          fileId: parseInt(data.fileId),
          modelName: data.modelName,
          fileSize: data.fileSize,
          previewImage: data.previewImage,
        },
      })

      if (result?.id) {
        textSpan.textContent = '✓'
        setTimeout(() => { textSpan.textContent = 'Download via CSD'; btn.style.pointerEvents = 'auto' }, 2000)
      } else {
        textSpan.textContent = 'Download via CSD'
        btn.style.pointerEvents = 'auto'
      }
    } catch (err: any) {
      textSpan.textContent = 'Download via CSD'
      btn.style.pointerEvents = 'auto'
      const msg = err?.message || ''
      if (msg.includes('context') || msg.includes('invalidated')) {
        showToast('🔄 Расширение обновлено. Перезагрузи страницу и попробуй снова.', 'warning')
      }
    }
  })
}

function tryInject(): boolean {
  const data = extractPageData()
  if (data) {
    injectButton(data)
    return true
  }
  return false
}

function showToast(msg: string, type: 'success' | 'error' | 'warning') {
  const el = document.createElement('div')
  el.textContent = msg
  el.style.cssText = [
    'position:fixed', 'bottom:24px', 'right:24px', 'padding:12px 20px',
    'border-radius:8px', 'color:white', 'font-size:13px', 'font-weight:500',
    'z-index:99999', 'box-shadow:0 4px 12px rgba(0,0,0,0.3)',
    type === 'success' ? 'background:#10b981' : '',
    type === 'error' ? 'background:#ef4444' : '',
    type === 'warning' ? 'background:#f59e0b' : '',
  ].join(';')
  document.body.appendChild(el)
  setTimeout(() => { el.style.transition = 'opacity 0.3s'; el.style.opacity = '0'; setTimeout(() => el.remove(), 300) }, 5000)
}

function observePageChanges() {
  let attempts = 0
  const observer = new MutationObserver(() => {
    if (document.querySelector('.csd-btn')) return
    attempts++
    if (tryInject()) {
      observer.disconnect()
    } else if (attempts > 30) {
      observer.disconnect()
    }
  })
  observer.observe(document.body, { childList: true, subtree: true })
  setTimeout(() => observer.disconnect(), 30000)
}
