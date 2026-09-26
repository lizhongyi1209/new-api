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
| Default frontend | `web/src/` |
| Production configuration and release | `Dockerfile`, `docker-compose.yml`, `.github/workflows/ci.yml`, `scripts/deploy-production.sh` |

## Task plugins and marketplace

Aliases: 任务插件, 插件市场, 插件广场, 插件上传, 插件版本, 插件沙箱, 任务日志制品, 图片制品, 视频预览, `/api/plugin/task`, `/task-plugins`.

- Root-only plugin management routes: `router/api-router.go`; handlers: `controller/task_plugin.go`; persistence and migration: `model/task_plugin.go`, `model/main.go`; settings and runtime switch: `setting/task_plugin.go`, `model/option.go`, `common/init.go`.
- Plugin execution foundation: `pkg/jsplugin/`, independently buildable `relaykit/`, built-in plugin sources under `plugins/`, and task adaptor `relay/channel/task/jsplugin/`; the master switch defaults off. Generic submission/read/artifact routes: `router/task-router.go`; submit preparation: `middleware/task_plugin_submit.go`; submission and billing handoff: `relay/relay_task.go`, `controller/relay.go`; context-aware polling: `service/task_polling.go`. Build checks pass; end-to-end plugin execution remains unverified.
- Default frontend page and marketplace: `web/src/routes/_authenticated/task-plugins/index.tsx`, `features/task-plugins/`, `hooks/use-sidebar-data.ts` under `web/src/`.
- Plugin channel binding: `controller/channel.go` checks the `task_plugin.bind` permission and registry metadata; `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx` selects a plugin, while `features/channels/lib/channel-form.ts` persists its key in channel `setting` JSON. Local channel type 62 avoids the existing 61 collision.
- Task provenance and narrow plugin view: `model/task.go`, `dto/task_plugin.go`, `service/task_plugin_audit.go`, `service/task_plugin_view.go`, `controller/relay.go`, and `service/task_billing.go`. Plugin-safe channel selection: `service/channel_select.go`, `model/channel_cache.go`, `model/ability.go`, `middleware/distributor.go`. Artifact access and media proxy: `controller/task.go`, `controller/video_proxy.go`, `middleware/task_artifact_access.go`, `service/task_artifact_access.go`; ordinary image result projection and legacy video action recognition also live in `controller/task.go`, with regressions in `controller/task_generic_test.go`. Public address and access limits: `setting/system_setting/system_setting_old.go`, `setting/system_setting/task_artifact.go`.
- Task usage pricing: `setting/billing_setting/task_plugin_pricing.go` stores expressions by plugin key and model name in the existing options table; `model/task_plugin_pricing.go`, `controller/model_pricing_config.go`, and `router/api-router.go` validate and save them through versioned `PATCH /api/option/task_plugin_pricing`. Ordinary model prices remain in `model/model_pricing_config.go`; its saves reject plugin-only `u(...)` expressions. `model/pricing.go` exposes public provider variants and an ordinary-channel marker for shared model names. `relay/helper/billing_expr_request.go` freezes referenced scalar request conditions; `relay/relay_task.go` reserves quota using the selected plugin's expression; `service/task_polling.go` settles completion facts from the frozen snapshot. Default frontend editor/matrix: `web/src/features/system-settings/models/task-plugin-pricing-editor.tsx`, `task-usage-pricing-editor.tsx`, `task-pricing-matrix.tsx`; public model detail and examples: `web/src/features/pricing/components/model-details.tsx`, `lib/task-expr.ts`; log display: `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`. Task artifact preview/download: `web/src/features/usage-logs/components/task-artifacts.tsx` via `GET /api/task/:task_id/artifacts`, with attachment response in `controller/task.go` and `controller/video_proxy.go`. Focused storage/billing tests: `model/task_plugin_pricing_test.go`, `pkg/billingexpr/task_usage_settle_test.go`; end-to-end execution remains unverified.

## Channel health-check scheduling

Aliases: 渠道自动检测, 仅检测自动禁用渠道, 检测并发, auto_ban_only, channel_test_concurrency.

- Monitor mode/concurrency defaults and bounds: `setting/operation_setting/monitor_setting.go`; option write validation: `model/option.go`.
- Scheduled/manual task dispatch, channel selection, bounded worker pool and health-result handling: `controller/channel-test.go`; existing task runner: `controller/system_task_handlers.go` and `service/system_task.go`.
- Settings UI: `web/src/features/system-settings/models/routing-reliability-section.tsx`, with page defaults in `features/system-settings/models/index.tsx` and registry mapping in `features/system-settings/models/section-registry.tsx`.

## Model-name exclusions and affinity templates

Aliases: 模型名排除, thinking 后缀白名单, re: 模型规则, Codex 亲和性模板.

- Exact and `re:` model-name exclusions, plus real model IDs ending in effort words: `setting/model_setting/global.go`; OpenAI suffix guard: `setting/reasoning/suffix.go`. Settings copy: `web/src/features/system-settings/models/global-settings-card.tsx`.
- Codex CLI header pass-through templates: `setting/operation_setting/channel_affinity_setting.go` and `web/src/features/system-settings/general/channel-affinity/constants.ts`; saved rules remain persisted through existing option keys.

## Frontend request failures, public documents, and legacy console addresses

Aliases: 请求失败, 业务失败, 提示去重, 公共文档缓存, ETag, 旧地址兼容, `/console`.

- Error payload/cause traversal, cancellation, safe messages and business-success guard: `web/src/lib/server-error-message.ts`; per-error notification ownership: `web/src/lib/handle-server-error.ts`. Authentication error mappings and safe presentations remain compatible with `lib/secure-verification.ts`.
- Shared query/mutation error policy: `web/src/lib/query-client.ts`, initialized by `web/src/main.tsx`; Axios transport retains its raw response contract in `web/src/lib/http-client.ts`. Ordinary request failures are notified at the final query or local caller boundary; 401 keeps the existing session-expiry presentation. Batch coverage and later payment/security boundaries are recorded in `docs/reviews/ui-batch-3b-validation-2026-09-14.md`.
- Public document routes: `router/api-router.go`; handlers: `controller/misc.go`; content revalidation and response envelope: `controller/revalidated_response.go`; weak validators: `common/etag.go`. Only notice, about, home content, user agreement and privacy policy participate. Frontend callers: `lib/api.ts`, `features/about/api.ts`, `features/home/api.ts`, `features/legal/api.ts`; private requests retain no-store.
- Legacy non-login addresses: `web/src/lib/legacy-route.ts`, applied before route loading in `web/src/routes/__root.tsx`; query parameters and fragments are preserved. Login migration belongs to the security batch.
- Fresh edit-detail guards: `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`, `web/src/features/users/components/users-mutate-drawer.tsx`; display-target isolation: `web/src/features/usage-logs/components/dialogs/user-info-dialog.tsx`; unavailable permission catalogs remain distinct from successful empty catalogs in `features/users/api.ts`. Settings saves reject business failures in `features/system-settings/hooks/use-update-option.ts`. Optional diagnostics can suppress both query error notification and 500 redirect with query meta.
- Focused regression entry points: `controller/revalidated_response_test.go`, `web/src/lib/server-error-message.test.ts`, `web/src/lib/__tests__/server-error-notifications.test.ts`.

## Playground requests, message editing, and overview guide

Aliases: 在线测试, 停止生成, 流式回复, 消息编辑, 未保存提醒, 概览向导, `/playground`, `/dashboard/overview`.

- Frontend routes: `web/src/routes/_authenticated/playground/index.tsx`, `routes/_authenticated/dashboard/$section.tsx`; feature entries: `features/playground/index.tsx`, `features/dashboard/index.tsx` under `web/src/`.
- Existing server path: `POST /pg/chat/completions` in `router/relay-router.go`, guarded by UserAuth and Distribute; `controller/playground.go` builds user/temporary-token context and calls `controller/relay.go`. Provider and billing behavior stays in the existing relay/service chain.
- Request generation and buffered-message isolation: `web/src/features/playground/hooks/use-chat-handler.ts`; asynchronous SSE lifecycle: `hooks/use-stream-request.ts` under that feature. Stopping preserves buffered text, while superseded/unmounted requests cannot update the current reply. Credential refresh timing belongs to the security batch.
- Edit discard confirmation and dirty-refresh warning: `features/playground/components/message/playground-message-editor.tsx` under `web/src/`, using the shared ConfirmDialog. This does not guard all router navigation.
- Shared streaming text presentation: `web/src/components/ai-elements/response-fade.ts`, `response.tsx`, `response-renderer.tsx`, `response-renderer-inline.tsx`, `reasoning.tsx`, and the dedicated block in `styles/index.css`; body and reasoning parser IDs stay separate.
- Overview guide and header action: `web/src/features/dashboard/components/overview/overview-dashboard.tsx`; existing onboarding state remains in the dashboard hooks.
- Focused regressions: `features/playground/hooks/use-stream-request.test.ts`, `use-chat-handler.test.ts`, `components/message/__tests__/playground-message-editor.test.tsx` under `web/src/`. Evidence: `docs/reviews/ui-batch-3c-validation-2026-09-14.md`.

## Account security, verification, and audit logs

Aliases: 安全与访问, 登录会话, 统一验证, Passkey 验证, 2FA 验证, 访问令牌, 操作审计, Telegram OAuth, `/api/verify`, `/api/audit`.

- Security routes and authorization: `router/api-router.go`, `middleware/auth.go`, `middleware/audit.go`, `service/authz/`.
- Unified sensitive-operation proof flow: `controller/secure_verification.go`, `middleware/secure_verification.go`, `service/security_verification.go`, `service/auth_token.go`.
- Login verification and session lifecycle: `controller/login_verification.go`, `controller/auth_session.go`, `service/auth_session.go`, `model/user_session.go`.
- Password login encryption switch/key endpoint: `controller/user.go`, `controller/misc.go`; crypto and single-slot persistence: `common/password_crypto.go`, `model/password_crypto.go`; both migration paths register `LoginEncryptionKey` in `model/main.go`. Frontend wiring: `web/src/features/auth/api.ts`, `features/auth/lib/password-encryption.ts`, `features/auth/sign-in/components/user-auth-form.tsx` under that frontend. Session-result guards: `routes/(auth)/sign-in.tsx`, `routes/_authenticated/route.tsx`; signup verification consumption: `features/auth/hooks/use-email-verification.ts`, `features/auth/sign-up/components/sign-up-form.tsx`; shared expiry/stale callback isolation: `components/turnstile.tsx`. Acceptance: `docs/reviews/ui-batch-6a-validation-2026-09-14.md`.
- Access-token lifecycle and audits: `controller/access_token.go`, `controller/audit.go`, `middleware/audit.go`, `model/audit_log.go`, `model/user.go`.
- Account password policy/hash, email/binding operations, and Telegram OAuth: `common/account_password.go`, `controller/user.go`, `controller/email_binding.go`, `controller/custom_oauth.go`, `model/account_security.go`, `oauth/telegram.go`.
- GitHub OAuth stable numeric ID and legacy binding guard: `oauth/github.go`, `controller/oauth.go`, `model/user.go`; regression: `controller/oauth_legacy_binding_test.go`. Browser-bound OAuth flow and session checks: `controller/oauth_browser_binding_test.go`.
- Frontend entry points: `web/src/features/security/`, `web/src/features/auth/secure-verification/`, `web/src/features/usage-logs/audit/`.

## Passkey domains, enrollment and verification

Aliases: 多域名 Passkey, 兼容域名, RP ID, RPID, WebAuthn, 域名删除确认.

- Routes: `router/api-router.go`; main login/enrollment/step-up/status/reset: `controller/passkey.go`; additional login verification: `controller/login_verification.go`. Both login paths must be considered for domain selection.
- WebAuthn configuration and origin/RP resolution: `service/passkey/service.go`; challenge/security transaction payload and consumption: `service/passkey/session.go`; user adapter: `service/passkey/user.go`.
- Credential storage and registration/assertion/auth-version updates: `model/passkey.go`; nullable `rp_id` binds new registrations and learns legacy bindings only after valid signatures. Both migration paths register its model in `model/main.go`. Domain settings snapshots: `setting/system_setting/passkey.go`; selected-transaction WebAuthn configuration: `service/passkey/service.go`.
- Root domain preview/confirmation endpoint `PUT /api/option/passkey/domains`: `controller/option.go`; transaction, impact count, stale confirmation and credential-write serialization: `model/passkey_option.go`; ordinary/bulk option and cache loading integration: `model/option.go`. Domain audit excludes confirmation/credential values: `controller/audit.go`; focused contract tests: `controller/passkey_domains_test.go`.
- Default frontend: `web/src/features/system-settings/auth/passkey-section.tsx`, `features/auth/passkey/`, `features/auth/sign-in/components/user-auth-form.tsx`, `features/auth/secure-verification/`, `features/security/components/passkey-card.tsx`, and `lib/passkey.ts` under that frontend.
- Multi-domain implementation scope: `docs/reviews/ui-batch-6c-scope-2026-09-14.md`; backend/isolated SQLite progress: `docs/reviews/ui-batch-6c-backend-progress-2026-09-14.md`. Completed frontend and focused three-database/local runtime acceptance: `docs/reviews/ui-batch-6c-validation-2026-09-14.md`; final isolated candidate is 127.0.0.1:3356, with nullable legacy credential migration and confirmed domain removal. Production is not deployed.

## OAuth access policies and Chat credential imports

Aliases: OAuth 策略模板, 信任等级, 组织或角色, 拒绝提示, OAuth 浏览器绑定, Chat 配置链接, AQBot, `{aqbotConfig}`, `/chat2link`.

- Root-only provider configuration routes: `router/api-router.go`; CRUD and safe response projection: `controller/custom_oauth.go`; existing policy storage/validation: `model/custom_oauth_provider.go`. Trusted provider userinfo evaluation, missing/invalid numeric field rejection, and denial-message rendering: `oauth/generic.go`; regression: `oauth/access_policy_templates_test.go`.
- Login state creation/callbacks: `controller/oauth.go`. Login flows persist a browser-cookie binding hash inside the existing auth-flow payload and reject missing/mismatched cookies before provider requests; bind/verify retain authenticated session/proof checks. Secure mode applies `middleware/auth_origin.go` to state creation and uses a host-only HttpOnly browser cookie; configuration is in `common/session_cookie.go`. Focused contracts: `controller/oauth_browser_binding_test.go`, `controller/auth_flow_test.go`, `controller/telegram_test.go`.
- OAuth policy and denial-message examples: `web/src/features/system-settings/auth/custom-oauth/components/access-policy-templates.ts`, applied by `provider-form-dialog.tsx` in that directory. Login initialization errors: `web/src/features/auth/hooks/use-oauth-login.ts`; callback route: `web/src/routes/oauth/$provider.tsx`.
- Chat placeholders and existing client import protocols: `web/src/features/chat/lib/chat-links.ts`. Credential retrieval with in-flight user/session guards: `features/chat/hooks/use-active-chat-key.ts`; keyless Chat2Link navigation: `routes/_authenticated/chat2link.tsx`; explicit sidebar external-link actions: `components/layout/components/chat-presets-item.tsx`, all under that frontend.
- Playground fresh authentication: `web/src/features/playground/hooks/use-stream-request.ts`, `features/playground/api.ts`. Non-stream generation uses the existing single-use authorization preflight and cannot be replayed by 401 refresh. Frontend regressions: Chat `chat2link.test.tsx`, `lib/chat-links.test.ts`, `hooks/use-active-chat-key.test.tsx`; Playground `api-auth.test.ts`, `hooks/use-stream-auth.test.ts`; auth `hooks/use-oauth-login.test.ts`.

- Local validation and risk boundaries: `docs/reviews/ui-batch-6b-validation-2026-09-14.md`.

## Embedded frontend and build artifacts

Aliases: 内嵌前端, 单一前端, 静态资源, HTML/JS 哈希.

- Build configuration/entry template: `web/rsbuild.config.ts`, `web/index.html`; generated production assets: `web/dist/`. Go embeds the single bundle in `main.go`; static serving: `router/web-router.go`.
- `Dockerfile` runs the root `web/` build and copies `web/dist/` into the Go stage. `theme.frontend` no longer switches runtime bundles.
- Verify that generated HTML entry scripts/styles exist before embedding; a successful build exit alone does not prove generated files were updated.

## Model, vendor, and conflict-aware pricing management

Aliases: 模型管理, 厂商管理, 模型可见性, 元数据同步, 定价冲突, pricing sync, vendor merge, `/api/models`, `/api/vendors`, `/api/option/model_pricing`.

- Routes and controllers: `router/api-router.go`, `controller/model_meta.go`, `controller/model_sync.go`, `controller/model_pricing_config.go`, `controller/vendor_meta.go`, `controller/ratio_sync.go`.
- Storage, optimistic versions, and cross-database mutations: `model/model_meta.go`, `model/model_metadata_sync.go`, `model/model_pricing_config.go`, `model/vendor_management.go`.
- Frontend model/vendor workflows: `web/src/features/models/`, `web/src/features/model-pricing/`, `web/src/features/system-settings/models/upstream-ratio-sync.tsx`.

## Pricing input currency and readonly time conditions

Aliases: 定价输入币种, USD 换算, 当前时段预览, 时间条件, 定价编辑滚动, `/api/option/model_pricing`.

- Existing administrator pricing route/controller and versioned storage: `router/api-router.go`, `controller/model_pricing_config.go`, `model/model_pricing_config.go`; frontend save contract: `web/src/features/model-pricing/api.ts`, `pricing.ts`.
- USD-owned inputs, display-currency selection and browser preference: `web/src/features/model-pricing/currency.ts`, `pricing-amount-input.tsx`, `pricing-currency-selector.tsx`, `web/src/stores/pricing-preferences-store.ts`. Editor and shared scroll/footer: `web/src/features/system-settings/models/model-pricing-sheet.tsx`, `model-pricing-inputs.tsx`, `tiered-pricing-editor.tsx`; legacy local video prices remain supported.
- Readonly bounded AST and condition descriptions: `web/src/features/pricing/lib/billing-expression/`; backend capability guard lives in `parser.ts`. Supported time branches are adapted in `lib/time-price.ts` and used by `lib/dynamic-price.ts`, model cards/price/cache cells, details and the editor preview. Complex unsupported expressions retain their source.
- Shared visible-page minute clock: `web/src/features/pricing/hooks/use-billing-time.ts`; enabled explicitly on current-time previews and never on historical log breakdowns. Backend time functions and frozen billing contracts remain in `pkg/billingexpr/run.go`, `types.go`, `expr.md`.
- Focused save/precision and time-boundary regressions: `web/src/features/model-pricing/__tests__/editor-currency.test.tsx`, `__tests__/pricing.test.ts`, `web/src/features/pricing/lib/__tests__/time-rule-expr.test.ts`.

## Per-request expressions and image cache billing

Aliases: fixed(), 按次表达式, 混合计费, img_cr, 图片缓存计费, request_rules, billing_tokens.

- Expression validation, reserved identifiers and request-rule tracing: `pkg/billingexpr/compile.go`, `fixed.go`, `run.go`; additive snapshot/unit fields and quota conversion: `types.go`, `settle.go`. Design contract: `pkg/billingexpr/expr.md`.
- Administrator draft preview/conversion: `POST /api/option/model_pricing/preview` and `/convert` in `router/api-router.go`, `controller/model_pricing_config.go`, `model/model_pricing_config.go`, `model/model_pricing_conversion.go`; legacy image request multipliers: `model/legacy_dalle_pricing.go`. Drafts are detached and readonly; explicit saves retain optimistic versions. Detached completion defaults share `setting/ratio_setting/model_ratio.go:ResolveCompletionRatio`.
- Default frontend condition documents/source-span serialization: `web/src/features/pricing/lib/billing-expression/visual.ts`; conditional editor and request simulation: `features/system-settings/models/tiered-pricing-editor.tsx`, `visual-billing-document-editor.tsx`, `visual-condition-tree.tsx`, `request-simulation.tsx` under that frontend. Currency-owned prices reuse `features/model-pricing/pricing-amount-input.tsx`; conversion confirmation and live preview: `features/model-pricing/pricing-conversion-dialog.tsx`, `api.ts`, `features/system-settings/models/model-pricing-sheet.tsx`. The capability guard enables fixed/img_cr and continues blocking usage pricing. Focused contracts: `model/model_pricing_conversion_test.go`, frontend `billing-expression/visual.test.ts`; local acceptance: `docs/reviews/ui-batch-5b2-validation-2026-09-14.md`.
- Existing pricing save route/controller/storage: `router/api-router.go`, `controller/model_pricing_config.go`, `model/model_pricing_config.go`; expression save validation: `setting/billing_setting/tiered_billing.go`. Usage-derived `u(...)` pricing remains blocked until a configured usage schema is implemented.
- Pre-consume and frozen request input: `relay/helper/price.go`, `billing_expr_request.go`; retry/image reservation: `service/image_billing.go`, `tiered_settle.go`; synchronous text/audio settlement: `service/text_quota.go`, `quota.go`; shared unit/cache log metadata: `service/log_info_generate.go`.
- Cache detail DTO and presence-preserving copies: `dto/openai_response.go`; image/Responses normalization: `relay/channel/openai/relay_image.go`, `relay_responses.go`, `relay_responses_compact.go`. Explicit `img_cr` separates only validated overlap; legacy expressions retain their previous normalization.
- Async submission/frozen scalar projection: `controller/async_image.go`, `controller/generate_image.go`; typed OpenAI cache extraction and worker settlement: `service/task_billing.go`, `generate_image.go`, `async_image.go`. New task snapshots record units/quantities and use shared opt-in token normalization; historical snapshots retain their task normalization. Referenced literal non-credential headers are frozen into snapshots; dynamic/credential header expressions are rejected before async pre-consume. Native Gemini freezes canonical generation settings and validates shared output-token/count bounds. Gemini workers share modality/cache extraction. Audio request-unit/zero-usage settlement and ordinary/streaming/compact Responses output details are verified. Task saturation emits request-correlated warnings; `model/log.go` merges admin metadata in completion/quota/other updates so audit markers survive. Batch 5B-1 is locally complete; editor/detail integration remains in 5B-2/3. Focused new regressions: `pkg/billingexpr/fixed_test.go`, `service/tiered_image_cache_test.go`, `setting/billing_setting/tiered_billing_test.go`, `relay/channel/openai/image_cache_usage_test.go`, `service/tiered_async_billing_test.go`, `controller/async_billing_headers_test.go`, `service/tiered_gemini_usage_test.go`, `service/tiered_audio_billing_test.go`; non-admin saturation projection: `model/log_format_test.go`. Current evidence: `docs/reviews/ui-batch-5b1-progress-2026-09-14.md`, `ui-batch-5b1-async-progress-2026-09-14.md`, `ui-batch-5b1-gemini-progress-2026-09-14.md` and final acceptance `ui-batch-5b1-validation-2026-09-14.md` in the same directory.

## Model square and hourly performance health

Aliases: 模型广场, 模型价格, 24 小时健康条, 零价, `/pricing`, `/api/perf-metrics/summary`.

- Catalog route and price data: `router/api-router.go`, `controller/pricing.go`, `model/pricing.go`; default frontend: `web/src/features/pricing/index.tsx`, `components/model-card.tsx`, `components/pricing-toolbar.tsx`, `lib/price.ts` under that feature. Explicit zero prices remain distinct from missing values; unit/recharge switches affect display only.
- Task-plugin catalog metadata and independently priced provider variants come from the active `pkg/jsplugin` routing generation in `model/pricing.go`; `features/pricing/components/model-details.tsx` lets users switch between ordinary-channel and plugin prices when a model name is shared. `features/pricing/lib/filters.ts` and `components/pricing-sidebar.tsx` include plugin variants in the task-billing filter while keeping shared models in their ordinary token/request category. Plugin task execution has not had end-to-end acceptance.
- Fixed/image unit and image-cache prices share `features/pricing/lib/billing-expr.ts`, `billing-expression/display.ts`, `structure.ts`, `dynamic-price.ts`, `time-price.ts` and `components/dynamic-pricing-breakdown.tsx` under the default frontend. Historical log snapshots/normalized counts are read by `features/usage-logs/types.ts`, `lib/format.ts`, `components/dialogs/details-dialog.tsx`; no current-time recalculation is enabled in logs. Built-in expression fallback and legacy-zero/explicit-mode precedence: `setting/billing_setting/builtin_billing.go`, `tiered_billing.go`, `setting/ratio_setting/model_ratio.go`; admin readonly exposure: `controller/option.go`. Local acceptance: `docs/reviews/ui-batch-5b3-validation-2026-09-14.md`; focused contracts: `setting/billing_setting/builtin_billing_test.go`, frontend `features/pricing/lib/dynamic-price.test.ts`, `features/usage-logs/lib/format.test.ts`.
- Public/user performance authorization and active-group filtering: `router/api-router.go`, `controller/perf_metrics.go`; aggregation and storage: `pkg/perf_metrics/metrics.go`, `pkg/perf_metrics/types.go`, `model/perf_metric.go`, `setting/perf_metrics_setting/config.go`.
- Summary adds `recent_success_series` with 24 hourly timestamps and nullable success rates, plus server snapshot/window timestamps. Minute buckets are weighted by request count, the last bar is the current partial hour, and existing summary/three-sample fields remain unchanged. Request counts remain private.
- Frontend API/types: `web/src/features/performance-metrics/`; health display and shared polling: `web/src/features/pricing/components/model-perf-badge.tsx`, `components/model-card-grid.tsx`. Older backends without hourly series show gray bars rather than synthesizing data.
- Regression entry points: `pkg/perf_metrics/metrics_test.go`, `web/src/features/pricing/__tests__/model-cards.test.tsx`.

## Advanced-custom route splitting and management routes

Aliases: 高级自定义渠道, 路由分流, client model matching, 模型列表路由, 余额查询路由, `/v1/models`, `/v1/dashboard/billing/credit_grants`.

- Route schema, model/regex matching, fallback ordering, and validation: `dto/channel_settings.go`.
- Relay route selection and management request construction: `relay/channel/advancedcustom/adaptor.go`.
- Upstream model discovery and balance queries: `controller/channel_upstream_update.go`, `controller/channel-billing.go`.
- Visual editor: `web/src/features/channels/components/dialogs/advanced-custom-editor-dialog.tsx`, `web/src/features/channels/lib/advanced-custom.ts`.

## Channel form drafts and model discovery

Aliases: 渠道创建/编辑, 草稿模型发现, 内联选模, 分类选模, `/api/channel/fetch_models`.

- Default frontend form/save path: `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`, `hooks/use-channel-mutate-form.ts`, `lib/channel-form.ts` under that feature. Local provider numbers are authoritative in `constants.ts`; AdvancedCustom helpers re-export its number 59, while TencentVideo remains 58.
- Provider selection and configuration pages: `components/drawers/channel-provider-picker.tsx`, `components/drawers/channel-configuration.tsx`, `lib/channel-configuration.ts` under that feature; existing local types remain selectable, and configuration drafts stay mounted while switching pages or opening the provider picker. Validation switches to the owning page before focusing the field. Shared provider logo: `components/channel-type-logo.tsx`.
- Inline selection: `web/src/features/channels/components/channel-model-discovery.tsx`, `components/upstream-model-selection.tsx`, `lib/model-categories.ts` under that feature; changes update the form only. Connection changes/close invalidate and abort browser discovery without discarding selected models.
- Authorization: `router/channel-router.go`; unsaved POST discovery retains ChannelSensitiveWrite, saved GET discovery retains ChannelOperate. Frontend transport: `web/src/features/channels/api.ts`.
- Provider-specific address hints: read-authorized `GET /api/channel/default_base_urls` in `router/channel-router.go` / `controller/channel.go` returns current `constant.ChannelBaseURLs` without empty entries. Default frontend uses placeholders only and falls back locally on failure. Regression: `controller/channel_defaults_test.go`, `router/channel_router_test.go`.
- Mapping direction/help and visual/JSON drafts: `web/src/features/channels/components/model-mapping-editor.tsx`; its controlled-value feedback preserves unfinished rows, while external resets replace the mapping. Regression: `components/model-mapping-editor.test.tsx`, `components/drawers/channel-configuration.test.tsx` under the channel feature.
- POST preview builder: `controller/channel_model_preview.go`; handler `controller/channel.go`; shared model-list routing/HTTP behavior `controller/channel_upstream_update.go`. Draft optional fields distinguish omitted from explicit empty values; detached previews retain unknown settings, use enabled saved keys without advancing polling, and cannot refresh saved Codex credentials.
- Regression entries: `controller/channel_model_preview_test.go`, `web/src/features/channels/components/__tests__/upstream-model-selection.test.tsx`, `web/src/features/channels/lib/__tests__/channel-type-options.test.ts`.

## Redemption-code bulk operations

Aliases: 批量删除兑换码, 导出兑换码, redemption export.

- API and persistence: `router/api-router.go`, `controller/redemption.go`, `model/redemption.go`.
- Frontend selection, deletion, and export: `web/src/features/redemption-codes/components/data-table-bulk-actions.tsx`, `web/src/features/redemption-codes/components/redemptions-export-dialog.tsx`.

## Shared status query and usage-log group filtering

Aliases: 状态请求去重, status dedup, 日志分组筛选, log group filter.

- Shared cached status query and consumers: `web/src/lib/status-query.ts`, `web/src/hooks/use-status.ts`, `web/src/hooks/use-system-config.ts`.
- Group-aware log query and filter UI: `model/log.go`, `controller/log.go`, `web/src/features/usage-logs/components/common-logs-filter-bar.tsx`.

## System updates, quota details, and mobile filters

Aliases: 更新提醒, 忽略版本, GitHub 发布详情, 额度弹层, 手机筛选折叠.

- Administrator update entry and maintenance page: `web/src/features/system-update/`, `web/src/components/layout/components/app-header.tsx`, `public-header.tsx` in that directory, and `web/src/features/system-settings/maintenance/update-checker-section.tsx`. Reads public GitHub releases; custom version comparison remains unknown, ignore preferences are browser-local and isolated by user ID.
- Shared quota popover and user/key values: `web/src/components/quota-details-popover.tsx`, `web/src/features/users/components/user-quota-cell.tsx`, `web/src/features/keys/components/api-key-quota-cell.tsx`.
- Shared mobile filter panel: `web/src/components/data-table/toolbar/mobile-filter-panel.tsx`; consumers: channel table toolbar and `web/src/features/usage-logs/components/logs-filter-toolbar.tsx`.
- Structured task details: `web/src/features/usage-logs/components/dialogs/task-detail-dialog.tsx`, `web/src/features/usage-logs/api.ts`; self views use task-list fields only, administrator views additionally call existing `GET /api/task/:task_id/audit` in `router/api-router.go` / `controller/task.go`.
- Administrator task-log list: `web/src/features/usage-logs/api.ts` calls `GET /api/task/`; `router/api-router.go` registers both `/api/task/` and `/api/task` (for cached clients) with `AdminAuth`, handled by `controller.GetAllTask` in `controller/task.go`. Self list uses `GET /api/task/self`.
- Existing default/classic theme switching: `web/src/features/system-settings/general/system-info-section.tsx`, `setting/system_setting/theme.go`, `model/option.go`, `router/web-router.go`, `common/embed-file-system.go`; persists `theme.frontend` and selects the corresponding embedded assets. Default-frontend UI changes do not automatically port features to classic.

## Shared table visibility, compact pagination, and drawer popups

Aliases: 列显隐刷新, 手机紧凑分页, 抽屉内选择器, portal container.

- Table-row visibility: `web/src/components/data-table/core/data-table-row.tsx` captures visible column IDs before the memo comparison, so stable TanStack rows update when columns are hidden/restored. Regression: `data-table-row.test.tsx` in that directory.
- Compact pagination: `web/src/components/data-table/core/pagination.tsx`; `layout/data-table-page.tsx` exposes optional `compactPagination`; mobile common logs opt in at `features/usage-logs/components/usage-logs-table.tsx` under `web/src/`. Default pagination remains unchanged. Regression: `core/pagination.test.tsx` under the data-table directory.
- Drawer popup scope: `web/src/components/ui/portal-container.ts`, `drawer.tsx`, `select.tsx`, `combobox.tsx`. Vaul drawer content supplies its container to nested floating controls; existing outside-drawer/mobile compatibility remains. Regression: `ui/__tests__/portal-container.test.tsx` and `combobox.test.tsx` under `web/src/components/`.
- Common-log mobile summaries and inspect dialogs: `web/src/features/usage-logs/components/common-log-mobile-card.tsx`, composed by `usage-logs-mobile-card.tsx`. Uses existing model/timing badges, Dialog and CopyButton. Cards retain identity by log ID; hiding metadata also closes its selected detail. Existing task/drawing details stay separate. Regression: `common-log-mobile-card.test.tsx`.
- Log scope and column preferences: `web/src/features/usage-logs/components/usage-logs-provider.tsx` resolves self/admin/root; `usage-logs-table.tsx` separates query cache, placeholder data and local column preferences by category/access. Backend permissions remain in `router/api-router.go` and `controller/log.go`. Regression: `usage-logs-table.test.tsx`.

## Activity time and API-key mobile details

Aliases: 活动时间, 最近使用, 最近登录, 密钥手机卡片, 继承用户分组, 模型/IP 限制详情.

- Shared timestamp/activity rendering: `web/src/components/activity-time-cell.tsx`; key wrapper: `features/keys/components/api-key-timestamp-cell.tsx`; desktop consumers: `features/keys/components/api-keys-columns.tsx` and `features/users/components/users-columns.tsx` under `web/src/`. Nonpositive timestamps render a dash; key activity is relative, user activity is absolute. Regression: `components/activity-time-cell.test.tsx`.
- Key mobile cards: `web/src/features/keys/components/api-keys-table.tsx`; restriction details reuse `api-keys-cells.tsx` with click popovers on mobile and hover details on desktop. Disclosure/copy still uses the original ApiKeyCell and API-key provider. Regression: `components/__tests__/api-key-restrictions.test.tsx` under that feature.
- Inherited group display: `web/src/features/keys/components/api-key-group-cell.tsx`, shared `components/group-badge.tsx` and `components/data-table/core/truncated-cell.tsx` under `web/src/`. Empty/whitespace group displays inherited user group without an explicit multiplier; automatic-group animation remains. Regression: `api-key-group-cell.test.tsx` in the feature's component test directory.

## Wallet order history and subscription preference

Aliases: 钱包, 订单搜索, 充值记录, 订阅偏好, 仅订阅, 订阅优先, `/wallet`.

- Wallet entry: `web/src/features/wallet/index.tsx`; order requests/lifecycle: `hooks/use-billing-history.ts`, `api.ts` and `components/dialogs/billing-history-dialog.tsx` under that feature. The existing shared `web/src/hooks/use-debounce.ts` combines search drafts; request generations isolate old search/page responses and unmounts. Admin completion refreshes the current search.
- Existing server authorization/routes: `router/api-router.go`; self/all order reads and admin completion: `controller/topup.go`; storage: `model/topup.go`. Search/UI changes do not change crediting or database schema.
- Subscription preference display: `web/src/features/wallet/components/subscription-plans-card.tsx`; transport: `features/subscriptions/api.ts`; server preference persistence: `controller/subscription.go`. Actual funding semantics remain in `service/billing_session.go`: subscription_only does not fall back to the wallet, while subscription_first uses the wallet when there is no active subscription.
- Waffo quote/confirmation: `web/src/features/wallet/api.ts`, `hooks/use-payment.ts`, `hooks/use-waffo-payment.ts`, `lib/payment.ts`, `index.tsx`, `components/dialogs/payment-confirm-dialog.tsx` under that feature. The existing `/api/user/waffo/amount` and `/api/user/waffo/pay` routes in `router/api-router.go` call `controller/topup_waffo.go`; method indices remain server-validated. Wallet confirmation keeps an amount/method snapshot, rejects obsolete selections, and locks repeated submissions; existing quote-error/concurrency protection remains.
- Focused regression: `web/src/features/wallet/hooks/use-payment.test.ts`; earlier acceptance records: `docs/reviews/ui-batch-4a-validation-2026-09-14.md`, `ui-batch-4b-validation-2026-09-14.md` in the same directory.

## Image quantity validation and billing reservation

Aliases: 图片数量, 图片预扣费, image_count, parameters.n, image reservation, 图像重试退款.

- Public routes: `/v1/images/generations`, `/v1/images/edits` in `router/relay-router.go`; dispatch: `controller/relay.go`.
- Request quantities and provider parameter validation: `dto/openai_image.go`, `relay/helper/valid_request.go`.
- Final outbound quantity, overrides and retry preparation: `relay/image_handler.go`; Ali forwarding: `relay/channel/ali/image.go`, `relay/channel/ali/image_wan.go`.
- Per-attempt reservation: `service/image_billing.go`; atomic wallet reservation and full refunds: `service/billing_session.go`, `service/funding_source.go`.
- Expression quantity input and settlement: `relay/helper/billing_expr_request.go`, `pkg/billingexpr/`, `service/tiered_settle.go`.
- Contract tests: `relay/image_billing_test.go`, `relay/helper/openai_image_request_test.go`, `service/tiered_settle_test.go`.

## Image output strategy

Aliases: 图片输出策略, 媒体输出策略, 视频输出策略, image output strategy, media output strategy, 图片落盘, 视频存储, 本机临时图片, temporary image, CF 图片, Cloudflare 图片, ESA 图片, `/tmp/output`, `/async/v1/generateImage`.

### Behavior contract

- Channel `settings` JSON key: `image_output_strategy` (legacy name; applies to generated images and completed videos).
- Accepted values are `oss`, `r2`, legacy `local_temp`, `local_temp_cf`, `local_temp_esa`, and `passthrough`.
- The channel editor offers `passthrough`, `oss`, and `r2`. It does not offer local URL strategies for new selection; existing local values remain visible as legacy settings and are retained until the administrator chooses another strategy. An unset value behaves like `passthrough`. Explicit `oss` stores durable image/video outputs under the OSS bucket's `output/` prefix. Explicit `r2` also applies to both media types while preserving its provider-specific object prefixes; local temporary strategies remain image-only for legacy configurations.
- `local_temp_cf` stores the response image locally and returns `https://cf-api.o1key.com/tmp/output/<uuid>.<ext>`.
- `local_temp_esa` stores the same way and returns `https://api.o1key.cn/tmp/output/<uuid>.<ext>`.
- Legacy `local_temp` remains valid for compatibility. It uses `TEMP_STORAGE_PUBLIC_BASE_URL`, then `LOCAL_PUBLIC_BASE_URL`, then the Cloudflare domain.
- An unset strategy and explicit `passthrough` both preserve upstream image/video output. The explicit synchronous image query `?image_format=url` retains its compatibility R2 rewrite.
- Explicit CF/ESA strategies select their fixed public domain independently of the configurable legacy base URL.
- Files live under `${TEMP_STORAGE_DIR:-tmp}/output`; production Compose defaults the root to `/data/tmp`. Each file is available for 24 hours, expired reads return not found, and the cleanup task removes expired files.
- The strategy applies to OpenAI-compatible image JSON/stream responses, Gemini inline-image responses, and the unified asynchronous `POST /async/v1/generateImage` result path.
- Plugin task artifact links use the separate `TaskPublicAddress` option (`setting/system_setting/system_setting_old.go`, `model/option.go`, `controller/option.go`, `service/task_artifact_access.go`, and the site settings form). It falls back to `ServerAddress` and does not override `settings.image_output_strategy`. The task artifact serving route is still under development.

### Entry points

| Concern | Source entry |
| --- | --- |
| Strategy constants, validation, temporary/storage classification | `dto/channel_settings.go` |
| Provider selection, object-storage upload, and CF/ESA domain dispatch | `service/storage.go` |
| Completed video output-strategy application in background polling | `service/task_polling.go` |
| Completed Gemini/Vertex video output-strategy application during real-time lookup | `relay/relay_task.go` |
| Stored video output URL preference for the public content proxy | `controller/video_proxy.go`, `controller/video_proxy_gemini.go` |
| Local file validation, atomic write, URL construction, 24-hour expiry and deletion | `service/temporary_image.go` |
| Periodic expiry cleanup | `service/temporary_image_cleanup_task.go` |
| Public `GET`/`HEAD /tmp/output/:filename` route | `router/main.go` |
| Temporary image response and cache headers | `controller/temporary_image.go` |
| OpenAI-compatible JSON and stream rewriting | `relay/channel/openai/relay_image.go` |
| Gemini inline-data rewriting | `relay/channel/gemini/relay-gemini-native.go` |
| Unified async image route | `router/relay-router.go` |
| Unified async image request controller | `controller/generate_image.go` |
| Async result conversion and output-strategy application | `service/generate_image.go` |
| Channel editor strategy control | `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx` |
| Channel form parses and saves strategy in `settings` JSON | `web/src/features/channels/lib/channel-form.ts` |
| Public base URL and persistent storage defaults | `docker-compose.yml` |

### Regression coverage

- `dto/channel_settings_test.go`
- `service/temporary_image_test.go`
- `service/storage_video_test.go`
- `controller/temporary_image_test.go`
- `relay/channel/openai/relay_image_temporary_storage_test.go`
- `relay/channel/gemini/relay_gemini_temporary_storage_test.go`
- `service/generate_image_test.go`

When changing this feature, preserve legacy `local_temp`, verify both explicit domains, cover stream and non-stream response paths, and keep the 24-hour expiry behavior fail-closed.

## Unified image `fileData` input

Aliases: fileData 输入, Gemini fileData, 生图输入优化, Base64 转临时 URL, `/async/v1/generateImage` 参考图, GPT Image 2.5, Sunburst, Flare.

### Behavior contract

- `/async/v1/generateImage` accepts legacy string items plus explicit `inlineData` and `fileData` objects in `images`.
- Models whose names start with `gpt-image` accept optional `background` values `auto` and `transparent`; Banana/Gemini image models reject the parameter. Transparent output requires `png` or `webp`, and the option is forwarded through both OpenAI image generations and edits requests.
- `/async/v1/generateImage`, `/v1/images/generations`, and `/v1/images/edits` accept the optional `moderation` values `auto` and `low` only for model names starting with `gpt-image`; the field is forwarded through OpenAI image generation JSON and edit multipart requests.
- `/async/v1/generateImage` accepts `quality=xhigh|max` for GPT Image 2.5 Sunburst and Flare model names, including their `-sp` and `-sd` aliases; earlier models reject those values.
- Channel `settings` JSON key `gemini_file_data_enabled` defaults to `false` and is only an upstream capability declaration. Gemini and Vertex channel editors expose it under Other Settings.
- With the setting disabled, Gemini reference URLs are downloaded and sent as `inlineData`, preserving the legacy behavior.
- With the setting enabled, explicit `fileData` and legacy URLs with a known image extension are sent as `fileData` without downloading the image body. Inline/Base64 images are validated and uploaded to OSS under `tmp/input/`, then exposed through the configured object-storage public URL.
- Unknown URL MIME types fall back to the legacy download-to-inline path. Object-storage failures fall back to `inlineData` only when the resulting Gemini request remains within the 20 MiB upstream limit.
- Input preparation logs contain only format/timing/size summaries and never include Base64 payloads or signed URL query parameters.

### Entry points

| Concern | Source entry |
| --- | --- |
| Mixed string/object input DTO | `dto/generate_image.go` |
| Input validation, route dispatch, and fallback body limit | `service/generate_image.go` |
| Gemini `inlineData`/`fileData` preparation and timing summary | `service/async_image.go` |
| Submission-time channel capability selection and input preparation log | `controller/generate_image.go` |
| Channel capability setting | `dto/channel_settings.go` |
| Reused validated OSS `tmp/input/` storage | `service/temporary_upload.go`, `service/storage.go` |
| Channel editor option and form persistence | `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`, `web/src/features/channels/lib/channel-form.ts` |
| Public API documentation | Source: `docs/api-doc.html`; independent Nano Banana, GPT Image, and Seedream sections at `#image-nano-banana`, `#image-gpt-image`, and `#image-seedream`, above Seedance; served directly by Nginx, not embedded in Go or Docker; publish updates with `scripts/deploy-api-doc.sh`; Nginx locations: `deploy/nginx/api-doc-locations.conf`; public routes: `/docs/`, `/docs/api-doc`, `/docs/download` |
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
- `layer_decomposition: true` requires exactly one input image, is forwarded as a top-level upstream field, and preserves layer metadata (`z_index`, `bounding_box`, `name`, `description`) in asynchronous task results.
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
| Admin usage-log column and detail table | `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`, `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` |

## Asset and material upload

Aliases: 素材上传, 文件上传, 上传素材, asset upload, element image upload, presign, R2 upload, local upload, Seedance asset, 上传管理.

There are several distinct upload flows. Identify the required contract before editing:

| Flow | HTTP/UI entry | Backend and frontend source entries |
| --- | --- | --- |
| Authenticated temporary attachment upload | `POST /v1/o1key/uploads`; legacy local files: `GET`/`HEAD /tmp/input/:filename` | Route: `router/relay-router.go`; multipart controller and public response: `controller/temporary_upload.go`; content validation, 20 MiB bound, OSS/R2 upload, object cleanup, and legacy local reads: `service/temporary_upload.go`, `service/temporary_image_cleanup_task.go`, `service/storage.go`; downstream integration guide: `docs/api-doc.html#temporary-upload`; contract tests: `service/temporary_upload_test.go`, `controller/temporary_upload_test.go`, `router/temporary_upload_router_test.go` |
| Authenticated object-storage upload | `POST /v1/storage/presign` | Route: `router/relay-router.go`; validation/controller: `controller/storage.go`; host-based R2/OSS presign selection: `service/storage.go` |
| Explicit OSS-compatible presign | `POST /v1/storage/oss/presign` | `router/relay-router.go`, `controller/storage.go`, `service/storage.go` |
| Legacy direct object upload | `POST /v1/storage/local/upload?object_key=uploads/...` | `router/relay-router.go`, `controller/storage.go`, `service/storage.go`; new files are written to OSS under `uploads/oss/` (R2 when OSS is administratively disabled). Existing local files remain served by `router/main.go` at `/upload/*` until removed. |
| Kling/Tencent element reference-image upload | `POST /api/element/kling/upload` and `POST /kling/v1/general/upload` | Routes: `router/api-router.go`, `router/video-router.go`; multipart validation/compression/upload: `controller/aigc_element.go`; image compression: `service/image_resize.go`; final storage dispatch: `service/storage.go` |
| Seedance unified asset creation from a public URL | `POST /v1/seedance/assets` with `type=hc`, `type=df`, or `type=doubao`; status: `GET /v1/seedance/assets/:asset_id?type=...`; HC uses `/v1/sd/assets*`, DF uses `/v1/sd-5/assets*`, Doubao MAX uses `/v2/db-sd-max/assets*`; the Doubao-native `/v2/db-sd-max/assets*` proxy is also exposed | Routes: `router/video-router.go`; stable request dispatcher and upstream proxy: `controller/seedance_asset_proxy.go`; model-based automatic conversion: `relay/channel/task/serviceinference/adaptor.go`; downstream integration guide: `docs/seedance-downstream-integration.md` |
| ServiceInference automatic image-to-asset conversion during video relay | No separate client upload endpoint; public image URLs in the generation request are converted automatically | `relay/channel/task/serviceinference/adaptor.go`; channel asset settings: `dto/channel_settings.go` |
| Admin upload inventory and cleanup | Frontend `/upload-management`; APIs under `/api/upload-management/*` | Route/API: `router/api-router.go`, `controller/upload.go`; frontend route: `web/src/routes/_authenticated/upload-management/index.tsx`; UI/API client: `web/src/features/upload-management/` |

Important boundaries:

- `POST /v1/o1key/uploads` accepts one `multipart/form-data` field named `file` and rejects additional file parts with HTTP 400. It supports PNG/JPEG/WebP images, MP3/WAV/M4A audio, MP4/MOV video, and PDF/TXT/MD documents up to 20 MiB, and stores new inputs under the object-storage `tmp/input/` prefix. The response URL uses the configured Aliyun OSS public base (R2 when OSS is administratively disabled), independent of the request host. Object cleanup is scheduled after 24 hours; public caches may outlive that time. Existing local `/tmp/input/` files retain their legacy read and cleanup path during the transition.
- Temporary attachments use UUID filenames and content-based type checks. Executable, archive-only, HTML, SVG, and extension/content mismatches are rejected. New non-media objects use `Content-Disposition: attachment`; legacy local non-media responses retain `nosniff` and a restrictive content security policy.
- Presign endpoints create short-lived upload authorization; the client still uploads the bytes to the returned upload URL.
- Browser uploads to Aliyun OSS require bucket CORS to allow `PUT` and the signed request headers. The `o1key-client` bucket uses `AllowedOrigins=*`, `AllowedMethods=PUT,GET,HEAD`, `AllowedHeaders=*`, and a 300-second preflight cache; presigned URLs remain required for writes.
- `UploadAigcElementImage` is a multipart convenience endpoint for element reference images and is not the generic presign API.
- `POST /v1/seedance/assets` accepts an already public HTTPS URL and creates an upstream asset; it does not receive raw multipart file bytes.
- Upload management lists and deletes legacy local upload inventory; it is not the object-upload entry point or an OSS inventory.
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
- Seedance HC public model names retain their mapping to the corresponding `dreamina-*-max` upstream names. The four `doubao-seedance-*-max` models are additionally exposed as direct public models on the same type-60 channel without model mapping.
- Dreamina MAX and Doubao MAX are classified separately but both currently submit and poll through the upstream `/v2/video/*` endpoints. Other ServiceInference models retain `/v1/video/*` behavior.
- MAX v2 media URLs are forwarded unchanged so the upstream task performs preparation. Doubao requests are restricted to the documented model/content/duration/resolution/ratio/generate_audio fields, including `aspect_ratio` alias normalization, while Dreamina request extensions remain unchanged. The upstream `preparing` state is normalized to public `in_progress` while preserved as `metadata.upstream_status`; the complete `prep` object is preserved as `metadata.prep`.
- The default channel-editor preset exposes the four HC names plus four directly callable Doubao MAX names and preserves the existing HC-to-Dreamina-MAX mapping.
- Route and relay-mode selection: `router/video-router.go`, `middleware/distributor.go`.
- Task submission/query response dispatch: `controller/relay.go`, `relay/relay_task.go`.
- ServiceInference request conversion, upstream polling, result parsing, and public video conversion: `relay/channel/task/serviceinference/adaptor.go`.
- Channel-editor HC-to-MAX defaults: `web/src/features/channels/lib/channel-type-config.ts`, `web/src/features/channels/lib/channel-form.ts`, `web/src/features/channels/components/drawers/channel-mutate-drawer.tsx`.
- Downstream reference and examples: `docs/api-doc.html#/v1/video/generations`, `docs/seedance-downstream-integration.md`.
- Response and mapping contract coverage: `relay/channel/task/serviceinference/adaptor_test.go`, `web/src/features/channels/lib/__tests__/channel-type-options.test.ts`.
