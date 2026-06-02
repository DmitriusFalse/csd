# Lora Manager API Reference

Source: [ComfyUI-Lora-Manager](https://github.com/willmiao/ComfyUI-Lora-Manager)

Базовый URL: `http://localhost:8188`

---

## REST API Architecture

Единый шаблон для всех типов моделей: `{type}` = `loras`, `checkpoints`, `embeddings`.

### Безопасность

- API работает только локально, без аутентификации
- CORS настроен только для локального использования
- file operations валидируют пути внутри корневых директорий моделей

### Ограничения

- Одновременно до 5 конкурентных загрузок
- Auto-organize — только одна операция за раз

---

## Common REST Endpoints

### Listing & Querying

| Endpoint | Method | Description |
|---|---|---|
| `/api/lm/{type}/list` | GET | Paginated model listing with filters |
| `/api/lm/{type}/top-tags` | GET | Most common tags across models |
| `/api/lm/{type}/base-models` | GET | List of base model types |
| `/api/lm/{type}/roots` | GET | Model root directories |
| `/api/lm/{type}/folders` | GET | Folder structure |
| `/api/lm/{type}/folder-tree` | GET | Hierarchical folder tree |
| `/api/lm/{type}/unified-folder-tree` | GET | Unified tree across roots |
| `/api/lm/{type}/find-duplicates` | GET | Detect duplicate models by hash |
| `/api/lm/{type}/find-filename-conflicts` | GET | Find naming conflicts |
| `/api/lm/{type}/verify-duplicates` | POST | Verify duplicate candidates |
| `/api/lm/{type}/metadata` | GET | Model metadata by file path |
| `/api/lm/{type}/model-description` | GET | Model description text |

**Query Parameters for `/list`:**
- Pagination: `page`, `page_size`, `sort_by`
- Filters: `folder`, `favorites_only`, `update_available_only`
- Search: `search`, `fuzzy`, `search_filename`, `search_modelname`, `search_tags`, `search_creator`, `recursive`
- Tags: `tag_include[]`, `tag_exclude[]`, `tag_logic`
- Base models: `base_model[]`
- License: `credit_required`, `allow_selling_generated_content`
- Model types: `model_type[]`

### Model Management

| Endpoint | Method | Description |
|---|---|---|
| `/api/lm/{type}/delete` | POST | Delete a single model |
| `/api/lm/{type}/bulk-delete` | POST | Delete multiple models |
| `/api/lm/{type}/exclude` | POST | Hide model from library |
| `/api/lm/{type}/rename` | POST | Rename model file |
| `/api/lm/{type}/save-metadata` | POST | Update model metadata |
| `/api/lm/{type}/add-tags` | POST | Add tags to model |
| `/api/lm/{type}/replace-preview` | POST | Upload new preview image |

### File Operations

| Endpoint | Method | Description |
|---|---|---|
| `/api/lm/{type}/move_model` | POST | Move single model to new location |
| `/api/lm/{type}/move_models_bulk` | POST | Move multiple models |
| `/api/lm/{type}/auto-organize` | POST | Auto-organize models using templates |
| `/api/lm/{type}/auto-organize-progress` | GET | Get auto-organize progress |
| `/api/lm/{type}/cancel-task` | POST | Cancel running task |

### CivitAI Integration

| Endpoint | Method | Description |
|---|---|---|
| `/api/lm/{type}/fetch-civitai` | POST | Refresh metadata for single model |
| `/api/lm/{type}/fetch-all-civitai` | POST | Refresh metadata for all models |
| `/api/lm/{type}/relink-civitai` | POST | Re-link model to CivitAI |
| `/api/lm/{type}/civitai/versions/{modelId}` | GET | Get available versions |
| `/api/lm/{type}/updates/refresh` | POST | Check for model updates |
| `/api/lm/{type}/updates/status` | GET | Get update status |
| `/api/lm/{type}/updates/versions/{modelId}` | GET | Get update versions |
| `/api/lm/{type}/updates/ignore` | POST | Ignore model update |
| `/api/lm/{type}/updates/ignore-version` | POST | Ignore version update |
| `/api/lm/{type}/updates/fetch-missing-license` | POST | Fetch missing license info |

### Scanning & Rebuilding

| Endpoint | Method | Description |
|---|---|---|
| `/api/lm/{type}/scan` | GET | Scan and refresh cache (query: `full_rebuild=false`) |

---

## Download API

Download endpoints unified across all model types.

| Endpoint | Method | Description |
|---|---|---|
| `/api/lm/download-model` | POST | Download model from CivitAI |
| `/api/lm/download-model-get` | GET | Download via GET (alternative) |
| `/api/lm/cancel-download-get` | GET | Cancel GET download |
| `/api/lm/download-progress` | GET | Get download progress |
| `/api/lm/force-download-example-images` | POST | Download example images |

---

## LoRA-Specific Endpoints

`/api/lm/loras/*`

| Endpoint | Method | Description |
|---|---|---|
| `/api/lm/loras/letter-counts` | GET | Count of LoRAs per alphabet letter |
| `/api/lm/loras/get-trigger-words` | GET | Get trigger words by name |
| `/api/lm/loras/usage-tips-by-path` | GET | Get usage tips by path |
| `/api/lm/loras/random-sample` | POST | Get random LoRA sample |
| `/api/lm/loras/cycler-list` | POST | Get filtered LoRA list for cycler |
| `/api/lm/loras/get_trigger_words` | POST | ComfyUI node integration |

---

## Checkpoint-Specific Endpoints

`/api/lm/checkpoints/*`

| Endpoint | Method | Description |
|---|---|---|
| `/api/lm/checkpoints/info/{name}` | GET | Get checkpoint info by name |
| `/api/lm/checkpoints/checkpoints_roots` | GET | Get checkpoint root directories |
| `/api/lm/checkpoints/unet_roots` | GET | Get unet root directories |

---

## Embedding-Specific Endpoints

`/api/lm/embeddings/*`

| Endpoint | Method | Description |
|---|---|---|
| `/api/lm/embeddings/info/{name}` | GET | Get embedding info by name |

---

## WebSocket API

Real-time progress updates для долгих операций.

### Endpoints

| Endpoint | Description | Cached |
|---|---|---|
| `/ws/init-progress` | Initialization progress per page type | Yes (per pageType) |
| `/ws/download-progress?id={downloadId}` | Download progress per download | Yes (5 min after disconnect) |
| `/ws/fetch-progress` | Metadata fetch progress | No |

### Init Progress Messages

URL: `ws://localhost:8188/ws/init-progress`

States:
- `loading` — initial cache loading
- `processing` — scanning files or building cache
- `completed` — initialization complete

### Download Progress Messages

URL: `ws://localhost:8188/ws/download-progress?id={downloadId}`

Кеширование: completed downloads cleanup через 5 мин после отключения, старые записи (>24ч) удаляются периодически.

### Auto-Organize Progress

Broadcast всем подключённым клиентам через `/ws` (general broadcast). Только одна операция за раз (`_auto_organize_lock`).

### Cache Health Warnings

Broadcast всем клиентам при обнаружении повреждения кэша.

---

## Client API Libraries (JavaScript)

`BaseModelApiClient` — общий класс для всех типов моделей. Создание:

```js
new BaseModelApiClient({ type: 'loras', baseUrl: '/api/lm' })
```

### Model-Specific Clients

- `LoraApiClient` — наследует BaseModelApiClient, добавляет методы для LoRA-специфичных эндпоинтов
- `CheckpointApiClient` — наследует BaseModelApiClient, добавляет методы для checkpoint-специфичных эндпоинтов
