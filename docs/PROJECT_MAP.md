# PROJECT_MAP.md — Agent Feature Entry Index

This file is the first-stop navigation index for existing features. It records stable entry points and cross-layer call paths so agents can avoid repeating broad repository searches.

## Required lookup workflow

1. Before locating, inspecting, debugging, or changing an existing feature, search this file by the user's terminology and the aliases listed below.
2. Open the listed route/controller/service/relay/frontend/test files that match the task, then verify the map against the current implementation.
3. Use `rg` only when the feature is absent, ambiguous, or the mapped paths are stale.
4. When a stable feature entry point is added or moved, update this map in the same change. Keep entries concise and point to source files instead of duplicating implementation details.

## Architecture shortcuts

| Layer | Primary location |
| --- | --- |
| HTTP routes | `router/` |
| Request handlers | `controller/` |
| Business and storage logic | `service/` |
| Database access | `model/` |
| Provider response/request adaptation | `relay/` and `relay/channel/` |
| Shared request/response settings | `dto/` |
| Default frontend | `web/default/src/` |
| Production configuration and release | `docker-compose.yml`, `scripts/deploy-production.sh` |

## Image output strategy

Aliases: 图片输出策略, image output strategy, 图片落盘, 本机临时图片, temporary image, CF 图片, Cloudflare 图片, ESA 图片, `/tmp/output`, `/async/v1/generateImage`.

### Behavior contract

- Channel setting key: `other.image_output_strategy`.
- Accepted values are `oss`, `r2`, legacy `local_temp`, `local_temp_cf`, `local_temp_esa`, and `passthrough`.
- `local_temp_cf` stores the response image locally and returns `https://cf-api.o1key.com/tmp/output/<uuid>.<ext>`.
- `local_temp_esa` stores the same way and returns `https://api.o1key.cn/tmp/output/<uuid>.<ext>`.
- Legacy `local_temp` remains valid for compatibility. It uses `TEMP_STORAGE_PUBLIC_BASE_URL`, then `LOCAL_PUBLIC_BASE_URL`, then the Cloudflare domain. The default frontend maps this legacy value to `local_temp_cf` when editing a channel.
- An unset strategy does not enable local temporary storage; existing host-based/default response behavior is preserved.
- Explicit CF/ESA strategies select their fixed public domain independently of the configurable legacy base URL.
- Files live under `${TEMP_STORAGE_DIR:-tmp}/output`; production Compose defaults the root to `/data/tmp`. Each file is available for 24 hours, expired reads return not found, and the cleanup task removes expired files.
- The strategy applies to OpenAI-compatible image JSON/stream responses, Gemini inline-image responses, and the unified asynchronous `POST /async/v1/generateImage` result path.

### Entry points

| Concern | Source entry |
| --- | --- |
| Strategy constants, validation, temporary/storage classification | `dto/channel_settings.go` |
| Provider selection and CF/ESA domain dispatch | `service/storage.go` |
| Local file validation, atomic write, URL construction, 24-hour expiry and deletion | `service/temporary_image.go` |
| Periodic expiry cleanup | `service/temporary_image_cleanup_task.go` |
| Public `GET`/`HEAD /tmp/output/:filename` route | `router/main.go` |
| Temporary image response and cache headers | `controller/temporary_image.go` |
| OpenAI-compatible JSON and stream rewriting | `relay/channel/openai/relay_image.go` |
| Gemini inline-data rewriting | `relay/channel/gemini/relay-gemini-native.go` |
| Unified async image route | `router/relay-router.go` |
| Unified async image request controller | `controller/generate_image.go` |
| Async result conversion and output-strategy application | `service/generate_image.go` |
| Channel editor options | `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx` |
| Channel form schema and legacy-to-CF normalization | `web/default/src/features/channels/lib/channel-form.ts` |
| Frontend settings type | `web/default/src/features/channels/types.ts` |
| Public base URL and persistent storage defaults | `docker-compose.yml` |

### Regression coverage

- `dto/channel_settings_test.go`
- `service/temporary_image_test.go`
- `controller/temporary_image_test.go`
- `relay/channel/openai/relay_image_temporary_storage_test.go`
- `relay/channel/gemini/relay_gemini_temporary_storage_test.go`
- `service/generate_image_test.go`

When changing this feature, preserve legacy `local_temp`, verify both explicit domains, cover stream and non-stream response paths, and keep the 24-hour expiry behavior fail-closed.

## Unified image `fileData` input

Aliases: fileData 输入, Gemini fileData, 生图输入优化, Base64 转临时 URL, `/async/v1/generateImage` 参考图.

### Behavior contract

- `/async/v1/generateImage` accepts legacy string items plus explicit `inlineData` and `fileData` objects in `images`.
- Channel setting `other.gemini_file_data_enabled` defaults to `false` and is only an upstream capability declaration.
- With the setting disabled, Gemini reference URLs are downloaded and sent as `inlineData`, preserving the legacy behavior.
- With the setting enabled, explicit `fileData` and legacy URLs with a known image extension are sent as `fileData` without downloading the image body. Inline/Base64 images are atomically stored under `${TEMP_STORAGE_DIR:-tmp}/input` and exposed through the CF `/tmp/input` URL.
- Unknown URL MIME types fall back to the legacy download-to-inline path. Local storage failures fall back to `inlineData` only when the resulting Gemini request remains within the 20 MiB upstream limit.
- Input preparation logs contain only format/timing/size summaries and never include Base64 payloads or signed URL query parameters.

### Entry points

| Concern | Source entry |
| --- | --- |
| Mixed string/object input DTO | `dto/generate_image.go` |
| Input validation, route dispatch, and fallback body limit | `service/generate_image.go` |
| Gemini `inlineData`/`fileData` preparation and timing summary | `service/async_image.go` |
| Submission-time channel capability selection and input preparation log | `controller/generate_image.go` |
| Channel capability setting | `dto/channel_settings.go` |
| Reused atomic `/tmp/input` storage | `service/temporary_upload.go` |
| Public API documentation route and embedded source | `GET`/`HEAD /docs/api-doc`; route: `router/web-router.go`; embedded source wiring: `main.go`; page source: `docs/api-doc.html#/async/v1/generateImage` |

### Regression coverage

- `dto/generate_image_test.go`
- `service/generate_image_file_data_test.go`
- `controller/generate_image_test.go`
- `dto/channel_settings_test.go`

## Asset and material upload

Aliases: 素材上传, 文件上传, 上传素材, asset upload, element image upload, presign, R2 upload, local upload, Seedance asset, 上传管理.

There are several distinct upload flows. Identify the required contract before editing:

| Flow | HTTP/UI entry | Backend and frontend source entries |
| --- | --- | --- |
| Authenticated temporary attachment upload | `POST /v1/o1key/uploads`; public file: `GET`/`HEAD /tmp/input/:filename` | Route: `router/relay-router.go`; multipart controller and public response: `controller/temporary_upload.go`; content validation, atomic local storage, CF/ESA URL mapping, 24-hour expiry and cleanup: `service/temporary_upload.go`, `service/temporary_image_cleanup_task.go`; contract tests: `service/temporary_upload_test.go`, `controller/temporary_upload_test.go`, `router/temporary_upload_router_test.go` |
| Authenticated object-storage upload | `POST /v1/storage/presign` | Route: `router/relay-router.go`; validation/controller: `controller/storage.go`; host-based R2/OSS/local presign selection: `service/storage.go` |
| Explicit OSS-compatible presign | `POST /v1/storage/oss/presign` | `router/relay-router.go`, `controller/storage.go`, `service/storage.go` |
| Direct local object upload | `POST /v1/storage/local/upload?object_key=uploads/...` | `router/relay-router.go`, `controller/storage.go`; public files are served by `router/main.go` at `/upload/*` |
| Signed downstream R2 upload | `POST /v1/storage/public/presign` | Route: `router/relay-router.go`; signature/origin/rate-limit checks: `controller/storage_public.go`; R2 presign: `service/storage.go`; admin whitelist UI: `web/default/src/features/system-settings/security/r2-public-upload-section.tsx` |
| Kling/Tencent element reference-image upload | `POST /api/element/kling/upload` and `POST /kling/v1/general/upload` | Routes: `router/api-router.go`, `router/video-router.go`; multipart validation/compression/upload: `controller/aigc_element.go`; image compression: `service/image_resize.go`; final storage dispatch: `service/storage.go` |
| Seedance unified asset creation from a public URL | `POST /v1/seedance/assets` with `type=hc` or `type=df`; status: `GET /v1/seedance/assets/:asset_id?type=hc|df`; HC uses `/v1/sd/assets*`, DF uses `/v1/sd-5/assets*` | Routes: `router/video-router.go`; stable request dispatcher and upstream proxy: `controller/seedance_asset_proxy.go`; model-based automatic conversion: `relay/channel/task/serviceinference/adaptor.go`; downstream integration guide: `docs/seedance-downstream-integration.md` |
| ServiceInference automatic image-to-asset conversion during video relay | No separate client upload endpoint; public image URLs in the generation request are converted automatically | `relay/channel/task/serviceinference/adaptor.go`; channel asset settings: `dto/channel_settings.go` |
| Admin upload inventory and cleanup | Frontend `/upload-management`; APIs under `/api/upload-management/*` | Route/API: `router/api-router.go`, `controller/upload.go`; frontend route: `web/default/src/routes/_authenticated/upload-management/index.tsx`; UI/API client: `web/default/src/features/upload-management/` |

Important boundaries:

- `POST /v1/o1key/uploads` accepts one `multipart/form-data` field named `file`, supports validated image/audio/video/document formats up to 48 MiB, and stores it under `${TEMP_STORAGE_DIR:-tmp}/input` for 24 hours. Requests sent through the CF or ESA API hostname receive the matching fixed public hostname; unknown hosts use the configured temporary-storage base and are never reflected into the response URL.
- Temporary attachments use UUID filenames and content-based type checks. Executable, archive-only, HTML, SVG, and extension/content mismatches are rejected. Non-media responses are forced to download with `nosniff` and a restrictive content security policy.
- Presign endpoints create short-lived upload authorization; the client still uploads the bytes to the returned upload URL.
- `UploadAigcElementImage` is a multipart convenience endpoint for element reference images and is not the generic presign API.
- `POST /v1/seedance/assets` accepts an already public HTTPS URL and creates an upstream asset; it does not receive raw multipart file bytes.
- Upload management lists and deletes local upload inventory; it is not the object-upload entry point.
- Storage provider routing, size/type validation, authentication, HMAC/origin checks, and public URL behavior are separate contracts. Trace the complete route → controller → service chain before changing one of them.
