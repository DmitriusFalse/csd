# Civitai Smart Downloader

Комплексное решение для автоматизированного скачивания моделей (LoRA, Checkpoints, Embeddings) с сайта Civitai напрямую в заданные директории.

## Архитектура

- **Модуль А (Browser Extension)** — расширение для Chromium-браузеров (Manifest V3, WXT + TypeScript)
- **Модуль Б (Local Bridge)** — локальный микросервис на Go для загрузки файлов и работы с файловой системой

## Быстрый старт

### 1. Серверная часть

```bash
go build -o csd-bridge.exe ./cmd/bridge/
./csd-bridge.exe
```

При первом запуске создастся `config.yaml` с настройками по умолчанию.

### 2. Расширение

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

### 3. Настройка

1. Укажите API-ключ Civitai (через popup расширения или в `config.yaml`)
2. Укажите корневой путь для сохранения моделей
3. Откройте страницу модели на civitai.com — появится кнопка «Скачать в Lora Manager»

## Конфигурация

См. `config.example.yaml`.

## Стек технологий

- **Go**: Fiber, zap, grab, energye/systray
- **Extension**: WXT, TypeScript, Manifest V3
- **Лицензия**: GPL 3.0
