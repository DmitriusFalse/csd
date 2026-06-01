import { defineConfig } from 'wxt'

export default defineConfig({
  manifestVersion: 3,
  name: 'Civitai Smart Downloader',
  version: '0.1.0',
  description: 'Download Civitai models directly to your local storage',
  permissions: ['storage'],
  hostPermissions: ['*://civitai.com/*', '*://civitai.red/*'],
  webExtension: {
    manifest: {
      minimum_chrome_version: '110',
    },
  },
})
