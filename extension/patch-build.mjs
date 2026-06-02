import { readFileSync, writeFileSync } from 'fs'
import { join, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const outDir = join(__dirname, '.output', 'chrome-mv3')

// Patch manifest.json
const manifestPath = join(outDir, 'manifest.json')
const manifest = JSON.parse(readFileSync(manifestPath, 'utf-8'))

manifest.permissions = ['storage']
manifest.host_permissions = [
  '*://civitai.com/*',
  '*://civitai.red/*',
  'http://127.0.0.1/*',
]
manifest.icons = { '128': 'icons/default.png' }
manifest.action = {
  ...manifest.action,
  default_icon: { '128': 'icons/default.png' },
}

writeFileSync(manifestPath, JSON.stringify(manifest, null, 2))
console.log('[patch] manifest.json updated with permissions, icons, action')

// Patch popup.html - remove crossorigin from script and link tags
const popupPath = join(outDir, 'popup.html')
let html = readFileSync(popupPath, 'utf-8')
html = html.replace(/crossorigin(="[^"]*")?/g, '')
writeFileSync(popupPath, html)
console.log('[patch] popup.html cleaned (removed crossorigin)')
