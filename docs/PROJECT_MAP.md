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
| Provider selection, object-storage upload, and CF/ESA domain dispatch | `service/storage.go` |
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
- Models whose names start with `gpt-image` accept optional `background` values `auto` and `transparent`; Banana/Gemini image models reject the parameter. Transparent output requires `png` or `webp`, and the option is forwarded through both OpenAI image generations and edits requests.
- `/async/v1/generateImage`, `/v1/images/generations`, and `/v1/images/edits` accept the optional `moderation` values `auto` and `low` only for model names starting with `gpt-image`; the field is forwarded through OpenAI image generation JSON and edit multipart requests.
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
| Channel editor setting | `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`, `web/default/src/features/channels/lib/channel-form.ts` |
| Public API documentation | Source: `docs/api-doc.html`; served directly by Nginx, not embedded in Go or Docker; publish updates with `scripts/deploy-api-doc.sh`; Nginx locations: `deploy/nginx/api-doc-locations.conf`; public routes: `/docs/`, `/docs/api-doc`, `/docs/download` |
| Operations and troubleshooting | `docs/operations/generate-image-filedata.md`, `docs/operations/generate-image-observability.md` |

### Regression coverage

- `dto/generate_image_test.go`
- `service/generate_image_file_data_test.go`
- `controller/generate_image_test.go`
- `dto/channel_settings_test.go`

## Seedream unified asynchronous image generation

Aliases: Seedream 5.0 Pro, `dola-seedream-5-0-pro-260628-ep`, `/async/v1/generateImage`, Seedream 图生图, TokenMart image.

- The public endpoint is the existing unified asynchronous task API. The gateway creates a local task, calls the upstream synchronous image endpoint in its worker, and returns the result through `/async/v1/tasks/:id`.
- Both text-to-image and image-to-image use upstream `POST /v1/images/generations` with JSON. Public `images` is mapped to the upstream singular `image` array without downloading URL references or switching to OpenAI multipart edits.
- Seedream request validation, model routing, worker dispatch, and result settlement: `service/generate_image.go`, `service/async_image.go`.
- Public DTO conversion and request-aware billing projection: `dto/generate_image.go`, `dto/async_image.go`, `controller/generate_image.go`.
- Async tiered settlement preserves the compact request body in `model.TaskBillingContext` and evaluates it in `service/task_billing.go`; image payloads and URLs are never stored in the billing snapshot.
- Public examples and parameter reference: `docs/api-doc.html#image-seedream`.
- Regression coverage: `service/generate_image_test.go`, `controller/generate_image_test.go`, `service/task_billing_test.go`.

## OpenAI Responses timing observability

Aliases: Responses 耗时, 首字耗时, 上游耗时, SSE 耗时, `/v1/responses`, responses timing.

### Behavior contract

- Only the public `POST /v1/responses` endpoint records `other.admin_info.responses_timing`; `/v1/responses/compact` and other relay routes are unchanged.
- The audit contains aggregate durations and byte counts only. Request/response bodies, URLs, credentials, and other user content are never stored.
- Client request receipt, local preparation, upstream connection/request write/header wait/response read, first SSE, and server-side downstream writes are measured separately.
- `upstream_total_ms` covers each upstream attempt from dispatch through the last upstream body read and is accumulated across channel retries.
- Downstream write timing measures server-side writes and flushes. It cannot prove when a proxy or the final client received the last byte.
- Timing data is nested under `admin_info`, so non-admin usage-log responses continue to strip it.

### Entry points

| Concern | Source entry |
| --- | --- |
| Route and relay dispatch | `router/relay-router.go`, `controller/relay.go` |
| Request conversion and timing lifecycle | `relay/responses_handler.go`, `relay/responses_timing.go` |
| Audit schema and retry-persistent relay state | `relay/common/responses_timing.go`, `relay/common/relay_info.go` |
| Shared upstream HTTP trace attachment | `relay/channel/api_request.go` |
| Usage-log admin metadata | `service/log_info_generate.go` |
| Admin usage-log column and detail table | `web/default/src/features/usage-logs/components/columns/common-logs-columns.tsx`, `web/default/src/features/usage-logs/components/dialogs/details-dialog.tsx` |

## Asset and material upload

Aliases: 素材上传, 文件上传, 上传素材, asset upload, element image upload, presign, R2 upload, local upload, Seedance asset, 上传管理.

There are several distinct upload flows. Identify the required contract before editing:

| Flow | HTTP/UI entry | Backend and frontend source entries |
| --- | --- | --- |
| Authenticated temporary attachment upload | `POST /v1/o1key/uploads`; public file: `GET`/`HEAD /tmp/input/:filename` | Route: `router/relay-router.go`; multipart controller and public response: `controller/temporary_upload.go`; content validation, atomic local storage, CF/ESA URL mapping, 24-hour expiry and cleanup: `service/temporary_upload.go`, `service/temporary_image_cleanup_task.go`; downstream integration guide: `docs/api-doc.html#temporary-upload`; contract tests: `service/temporary_upload_test.go`, `controller/temporary_upload_test.go`, `router/temporary_upload_router_test.go` |
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

## MiniMax H3 video generation

Aliases: MiniMax H3, H3 Max, MiniMax-H3, MiniMax-H3-Max, 海螺 H3, 尾帧视频, 多模态参考视频.

- Downstream submission uses `POST /v1/video/generations`; task lookup uses `GET /v1/videos/:task_id`.
- `MiniMax-H3` supports 768P/2K, 4-15 second T2V, first-frame/last-frame I2V, and multimodal reference generation. Reference inputs allow up to 9 images, 3 videos, and 3 audios, with at most 12 mixed media files in total.
- `MiniMax-H3-Max` supports 480P/768P, 5-15 second T2V and first-frame/last-frame I2V only. It does not accept multimodal reference image, video, or audio inputs.
- Both models use MiniMax V2 create/query endpoints. Query usage preserves output seconds, reference-video seconds, input-audio seconds, image count, and token fields.
- H3 billing uses the official CNY output/input rates and settles from returned usage. H3 Max charges output only; its input images are free.
- Provider model lists and endpoints: `relay/channel/task/hailuo/constants.go`, `relay/channel/minimax/constants.go`.
- Request validation, V2 response parsing, and public response conversion: `relay/channel/task/hailuo/v2.go`, `relay/channel/task/hailuo/adaptor.go`.
- Submit estimation and completion settlement: `relay/channel/task/hailuo/billing.go`; base ratio enablement: `setting/ratio_setting/model_ratio.go`.
- TokenMart/type-60 transport keeps the same downstream MiniMax request contract while mapping model IDs to `minimax-h3` / `minimax-h3-max`, submitting and polling through `/v1/video/*`, bypassing Seedance asset conversion, and settling from MiniMax's official RMB per-second/image prices through the configured USD/RMB conversion: `relay/channel/task/serviceinference/adaptor.go`, `relay/channel/task/serviceinference/minimax_h3.go`.
- Public downstream reference: `docs/api-doc.html#minimax-h3`.
- Regression coverage: `relay/channel/task/hailuo/v2_test.go`, `relay/channel/task/serviceinference/adaptor_test.go`.

## Seedance video generation

Aliases: Seedance 视频, Seedance task, 查询视频任务, `/v1/video/generations`, TokenMartSeedance, ServiceInference video, Seedance MAX, `/v2/video/generate`.

- Downstream submission uses `POST /v1/video/generations`; task lookup uses `GET /v1/video/generations/:task_id` with the public `task_...` ID returned at submission.
- ServiceInference tasks use the public OpenAI-style video object for both submission and lookup. Query states are `queued`, `in_progress`, `completed`, and `failed`; completed results expose URLs and usage under `metadata`, plus a top-level `result_url` compatibility alias for legacy clients, and must not expose internal task, channel, user, group, or quota fields.
- Seedance HC public model names can map to the corresponding `*-max` upstream names. MAX models submit and poll through the upstream `/v2/video/*` endpoints while all other ServiceInference models retain `/v1/video/*` behavior.
- MAX v2 media URLs are forwarded unchanged so the upstream task performs preparation. Its `preparing` state is normalized to public `in_progress` while preserved as `metadata.upstream_status`; the complete `prep` object is preserved as `metadata.prep`.
- The default channel-editor preset exposes the four HC names and fills their HC-to-MAX model mapping, keeping upstream MAX identifiers out of the public model list.
- Route and relay-mode selection: `router/video-router.go`, `middleware/distributor.go`.
- Task submission/query response dispatch: `controller/relay.go`, `relay/relay_task.go`.
- ServiceInference request conversion, upstream polling, result parsing, and public video conversion: `relay/channel/task/serviceinference/adaptor.go`.
- Channel-editor HC-to-MAX defaults: `web/default/src/features/channels/lib/channel-type-config.ts`, `web/default/src/features/channels/lib/channel-form.ts`, `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`.
- Downstream reference and examples: `docs/api-doc.html#/v1/video/generations`, `docs/seedance-downstream-integration.md`.
- Response and mapping contract coverage: `relay/channel/task/serviceinference/adaptor_test.go`, `web/default/src/features/channels/lib/__tests__/channel-type-config.test.ts`.
