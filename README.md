# Civitai Smart Downloader

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

---

## English

**Civitai Smart Downloader** is a complete solution for automated downloading of models (LoRA, Checkpoints, Embeddings, etc.) from Civitai directly to specified directories.

### Architecture

- **Module A (Browser Extension)** — Chromium extension (Manifest V3, WXT + TypeScript)
- **Module B (Local Bridge)** — local Go microservice for file downloads and filesystem operations

### Quick Start

#### 1. Server

```bash
go build -o csd-bridge.exe ./cmd/bridge/
./csd-bridge.exe
```

On first run, `config.yaml` will be created with default settings.

#### 2. Extension

```bash
cd extension
npm install
npm run build
```

Load from `extension/.output/chrome-mv3/` into Chrome:
1. Open `chrome://extensions`
2. Enable Developer mode
3. Click "Load unpacked"
4. Select `extension/.output/chrome-mv3/`

#### 3. Configuration

1. Set your Civitai API key (in popup or `config.yaml`)
2. Set the root save path for models
3. Open a model page on civitai.com — a "Download via CSD" button will appear

### i18n / Localization

The extension and server tray support English and Russian. Language is auto-detected from your browser/system language. You can switch manually:

- **Extension popup**: click `EN` / `RU` in the header
- **Server tray**: right-click → Language → choose English / Russian / Auto

### Donate

Support the project:

- [Boosty](https://boosty.to/sir.geronis/donate)
- [Patreon](https://www.patreon.com/16134050/join)

### Tech Stack

- **Go**: Fiber, zap, grab, energye/systray
- **Extension**: WXT, TypeScript, Manifest V3
- **License**: GPL 3.0

---

## Русский

**Civitai Smart Downloader** — комплексное решение для автоматизированного скачивания моделей (LoRA, Checkpoints, Embeddings и др.) с сайта Civitai напрямую в заданные директории.

### Архитектура

- **Модуль А (Browser Extension)** — расширение для Chromium-браузеров (Manifest V3, WXT + TypeScript)
- **Модуль Б (Local Bridge)** — локальный микросервис на Go для загрузки файлов и работы с файловой системой

### Быстрый старт

#### 1. Серверная часть

```bash
go build -o csd-bridge.exe ./cmd/bridge/
./csd-bridge.exe
```

При первом запуске создастся `config.yaml` с настройками по умолчанию.

#### 2. Расширение

```bash
cd extension
npm install
npm run build
```

Загрузите расширение из `extension/.output/chrome-mv3/` в Chrome:
1. Откройте `chrome://extensions`
2. Включите «Режим разработчика»
3. Нажмите «Загрузить распакованное расширение»
4. Выберите папку `extension/.output/chrome-mv3/`

#### 3. Настройка

1. Укажите API-ключ Civitai (через popup расширения или в `config.yaml`)
2. Укажите корневой путь для сохранения моделей
3. Откройте страницу модели на civitai.com — появится кнопка «Скачать через CSD»

### Локализация (i18n)

Расширение и трей сервера поддерживают английский и русский языки. Язык определяется автоматически из языка браузера/системы. Можно переключить вручную:

- **Попап расширения**: нажмите `EN` / `RU` в заголовке
- **Трей сервера**: правый клик → Language → выберите English / Russian / Auto

### Поддержать проект

❤️ Поддержать проект:

- [Boosty](https://boosty.to/sir.geronis/donate)
- [Patreon](https://www.patreon.com/16134050/join)

### Стек технологий

- **Go**: Fiber, zap, grab, energye/systray
- **Extension**: WXT, TypeScript, Manifest V3
- **Лицензия**: GPL 3.0
