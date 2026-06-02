import { defineConfig } from 'wxt'

export default defineConfig({
  manifestVersion: 3,
  manifest: {
    minimum_chrome_version: '110',
    permissions: ['storage'],
    host_permissions: [
      '*://civitai.com/*',
      '*://civitai.red/*',
      'http://127.0.0.1/*',
    ],
  },
})
