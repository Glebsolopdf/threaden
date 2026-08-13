# Вложения в сообщениях Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить безопасную синхронную загрузку изображений, видео, файлов и архивов в групповые сообщения с подписями, обработкой, квотами, TTL-очисткой и защитой диска.

**Architecture:** HTTP handler принимает JSON как раньше и multipart для вложений. Изолированный `internal/attachments` обрабатывает и временно хранит бинарные данные, а group/application слой создаёт сообщение и метаданные в SQLite-транзакции. Бинарные файлы лежат вне SQLite; выдача идёт через авторизованный endpoint.

**Tech Stack:** Go 1.26, Chi, SQLite через существующий store, стандартные архиваторы Go, `image`/`golang.org/x/image`, системный `ffmpeg`, Angular 21, TypeScript, RxJS.

## Global Constraints

- В одном сообщении максимум 3 вложения.
- Входное медиа максимум 10 МБ, архив максимум 5 МБ.
- Обработанное изображение или видео максимум 1 МБ.
- Активная квота пользователя 50 МБ, суточный лимит новых вложений 20 МБ.
- TTL вложения — 72 часа; общая квота хранилища — 5 ГБ.
- Неизвестные форматы, неверные сигнатуры, traversal, симлинки и архивные бомбы отклоняются.
- JSON-сообщения без вложений сохраняют существующий контракт.
- Ни один исходный файл не превышает 300 строк; каталог содержит максимум 5 исходных файлов.
- Production Docker image обязан содержать `ffmpeg`.

---

### Task 1: Конфигурация лимитов и доменные типы

**Files:**
- Create: `backend/internal/attachments/limits.go` (~100 lines) — лимиты, типы вложений и значения по умолчанию.
- Create: `backend/internal/attachments/errors.go` (~80 lines) — ошибки валидации, квоты, места и обработки.
- Modify: `backend/internal/config/config.go` — загрузка duration/bytes/int настроек вложений.
- Modify: `backend/internal/config/config_test.go` — defaults, overrides и rejection of invalid values.
- Modify: `backend/.env.example` — документированные environment variables.
- Modify: `backend/README.md` — таблица конфигурации и требование `ffmpeg`.

**Interfaces:**
- Produces `attachments.Limits` with `MaxInputMediaBytes`, `MaxArchiveBytes`, `MaxOutputMediaBytes`, `MaxFilesPerMessage`, `MaxUserStoredBytes`, `MaxUserDailyBytes`, `MaxTotalBytes`, `Retention`, `StorageDir`.
- Produces typed errors that HTTP mapping can convert to stable error codes.

- [ ] **Step 1: Write the failing tests** for default values, a valid override set, and rejection of zero/negative values.
- [ ] **Step 2: Run `cd backend && go test ./internal/config -run 'Test.*Attachment|TestLoad'`** and confirm the new assertions fail because fields do not exist.
- [ ] **Step 3: Implement the limits and config parsing** using the existing `positiveInt`, `bytesValue`, and `duration` conventions. Add defaults: 10 MiB, 5 MiB, 1 MiB, 3, 50 MiB, 20 MiB, 5 GiB, 72h.
- [ ] **Step 4: Run the focused config tests** and confirm they pass.
- [ ] **Step 5: Commit** with `git add backend/internal/attachments backend/internal/config backend/.env.example backend/README.md && git commit -m "feat: configure attachment limits"`.

### Task 2: SQLite attachment metadata and message model

**Files:**
- Create: `backend/internal/store/attachments/metadata.go` (~220 lines) — insert/list/get/delete metadata, quota sums, expiry and orphan queries.
- Create: `backend/internal/store/tests/attachments_test.go` (~220 lines) — metadata lifecycle, user/global quota queries and expiry selection.
- Create: `backend/internal/store/schema/migration21.go` (~70 lines) — `attachments` table, indexes, foreign keys and expiry index.
- Modify: `backend/internal/store/schema/migrations.go` — set `LatestVersion = 21` and register migration 21.
- Modify: `backend/internal/model/model.go` — add `Attachment` and `GroupMessage.Attachments`.
- Modify: `backend/internal/store/groupmessages/messages.go` — scan and write attachment references without changing existing message JSON behavior.
- Modify: `backend/internal/store/store.go` — expose attachment metadata operations and a transaction boundary.

**Interfaces:**
- `model.Attachment` contains `ID`, `MessageID`, `GroupID`, `OwnerID`, `Kind`, `Mime`, `Name`, `Size`, `Path`, `CreatedAt`, `ExpiresAt`.
- `attachments.Store` operations accept `context.Context`, use prepared values, and never expose filesystem paths to clients.
- `groupmessages.Add` remains compatible for system and text messages; a new `AddWithAttachments` writes message plus metadata in one transaction.

- [ ] **Step 1: Write failing store tests** for migration-created metadata, message attachment round-trip, per-user sum, global sum, expiry listing, and cascade deletion.
- [ ] **Step 2: Run `cd backend && go test ./internal/store/tests ./internal/store/sqlite -run Attachment`** and verify failure on the missing table/API.
- [ ] **Step 3: Add migration 21 and focused metadata methods** with foreign keys and indexes on `(owner_id, created_at)`, `(expires_at)`, and `(message_id)`.
- [ ] **Step 4: Extend model scanning and transactional message insertion** so `GET messages` returns attachments in stable creation order.
- [ ] **Step 5: Run `cd backend && go test ./internal/store/...`** and confirm all store tests pass.
- [ ] **Step 6: Commit** with `git add backend/internal/model backend/internal/store && git commit -m "feat: store attachment metadata"`.

### Task 3: Secure format detection and media processors

**Files:**
- Create: `backend/internal/attachments/detect.go` (~180 lines) — content sniffing, extension-independent kind detection and safe names.
- Create: `backend/internal/attachments/image/image.go` (~220 lines) — bounded decode and safe re-encode.
- Create: `backend/internal/attachments/video/ffmpeg.go` (~220 lines) — bounded command execution, input probing, transcoding and output validation.
- Create: `backend/internal/attachments/archive/archive.go` (~240 lines) — archive signature parsing and bomb/path/special-file checks without extraction.
- Create: `backend/internal/attachments/processor.go` (~220 lines) — processor orchestration and normalized `ProcessedFile` result.
- Create: `backend/internal/attachments/detect_test.go` (~220 lines) — valid signatures, mismatched extensions, corrupt files and unsafe names.
- Create: `backend/internal/attachments/archive/archive_test.go` (~240 lines) — traversal, symlink, special-file, entry-count, expanded-size and overflow cases.
- Create: `backend/internal/attachments/image/image_test.go` (~200 lines) — valid re-encoding, pixel bounds and output-size rejection.
- Create: `backend/internal/attachments/video/ffmpeg_test.go` (~180 lines) — fake executable/probe failures, timeout and output-size rejection.

**Interfaces:**
- `Processor.Process(ctx context.Context, src io.Reader, originalName string, inputSize int64) (ProcessedFile, error)`.
- `ProcessedFile` contains `Kind`, `Mime`, `DisplayName`, `Size`, and a temporary output path owned by the caller.
- `video.Runner` is the only boundary that knows the `ffmpeg` command; tests use a deterministic fake runner.

- [ ] **Step 1: Write failing tests** for each detector and processor boundary before production code.
- [ ] **Step 2: Run `cd backend && go test ./internal/attachments/...`** and confirm failures are caused by missing implementations.
- [ ] **Step 3: Implement signature detection and name sanitization**; never use the client filename as a storage path or MIME source.
- [ ] **Step 4: Implement image processing** with `image.DecodeConfig`, a maximum dimension/pixel budget, and deterministic server output.
- [ ] **Step 5: Implement archive validation** with integer-overflow-safe size accounting and explicit rejection of unsafe entry types.
- [ ] **Step 6: Implement the `ffmpeg` adapter** with context cancellation, no shell interpolation, fixed output arguments, a temporary output file, and final MIME/size validation.
- [ ] **Step 7: Run focused processor tests** and confirm all pass, including tests with misleading `.jpg` and archive extensions.
- [ ] **Step 8: Commit** with `git add backend/internal/attachments && git commit -m "feat: securely process attachment formats"`.

### Task 4: Quota-aware filesystem storage and cleanup

**Files:**
- Create: `backend/internal/attachments/storage/files.go` (~220 lines) — random IDs, private directory layout, atomic rename, safe cleanup.
- Create: `backend/internal/attachments/storage/quota.go` (~180 lines) — disk check, database totals, concurrent reservations and release.
- Create: `backend/internal/attachments/cleanup/cleanup.go` (~220 lines) — expired metadata/files and orphan reconciliation.
- Create: `backend/internal/attachments/storage/files_test.go` (~220 lines) — path safety, atomic write, rollback and orphan cleanup.
- Create: `backend/internal/attachments/storage/quota_test.go` (~200 lines) — user/global/disk limits and concurrent reservation behavior.
- Modify: `backend/internal/disk/disk.go` — expose the existing free-space check through the attachment storage dependency without duplicating syscall logic.
- Modify: `backend/internal/app/cleanup.go` — run attachment cleanup in the existing minute cleanup loop.
- Modify: `backend/cmd/api/main.go` — construct storage, processor and cleanup service; pass attachment root and limits.

**Interfaces:**
- `storage.Service.Prepare(ctx, userID string, files []*multipart.FileHeader) (Batch, error)` stages processed files and reserves quota.
- `Batch.Commit(ctx, messageID, groupID string) ([]model.Attachment, error)` writes metadata; `Batch.Rollback()` is idempotent.
- `cleanup.Service.RunOnce(ctx, now time.Time) error` deletes expired files and orphaned paths.

- [ ] **Step 1: Write failing tests** for atomic staging, rollback after a later file fails, free-space rejection, user/global quotas, and orphan cleanup.
- [ ] **Step 2: Run the focused storage tests** and confirm they fail before implementation.
- [ ] **Step 3: Implement storage under a configured private directory** using `os.OpenFile` limits, random IDs, `0600` files, `0700` directories, and atomic rename.
- [ ] **Step 4: Implement reservation accounting** so concurrent requests reserve the maximum allowed output before processing and always release on failure.
- [ ] **Step 5: Implement TTL/orphan cleanup** in bounded batches; tolerate an already-missing file while preserving metadata consistency.
- [ ] **Step 6: Wire cleanup and startup construction** into the existing lifecycle, preserving low-disk emergency cleanup behavior.
- [ ] **Step 7: Run `cd backend && go test ./internal/attachments/... ./internal/app/...`** and confirm all pass.
- [ ] **Step 8: Commit** with `git add backend/internal/attachments backend/internal/app backend/internal/disk backend/cmd/api && git commit -m "feat: add quota-aware attachment storage"`.

### Task 5: Group message application flow and HTTP API

**Files:**
- Create: `backend/internal/httpapi/groupchat/multipart.go` (~200 lines) — parse multipart fields, enforce file count/input limits, map attachment errors.
- Create: `backend/internal/httpapi/groupchat/multipart_test.go` (~220 lines) — body-only, file-only, caption, three-file limit, oversize and rollback requests.
- Modify: `backend/internal/httpapi/groupchat/handler.go` — dispatch JSON vs multipart without changing existing JSON decoding.
- Modify: `backend/internal/httpapi/router.go` — register authorized attachment download route and map domain errors to stable status/codes.
- Modify: `backend/internal/groups/messages.go` — permit body-only or attachment-only messages, run antispam on captions, and publish attachment metadata.
- Modify: `backend/internal/publicview/publicview.go` — expose only public attachment fields and generated download URL.
- Modify: `backend/internal/integration/security_test.go` — add cross-group access, path traversal and wrong-content-type regression cases.
- Modify: `backend/README.md` — document multipart contract and download endpoint.

**Interfaces:**
- Multipart handler calls `attachments.Service.Prepare`, then `groups.Service.SendWithAttachments(ctx, groupID, user, body, replyToID, batch, idempotencyKey)`.
- Download handler calls `attachments.Service.OpenForGroup(ctx, attachmentID, viewer, groupID)` and streams the stored server MIME.
- `SendWithAttachments` commits the message and batch together; on any failure it invokes `Rollback` and publishes no event.

- [ ] **Step 1: Write failing HTTP/integration tests** for all accepted request modes and security failures.
- [ ] **Step 2: Run `cd backend && go test ./internal/httpapi/groupchat ./internal/integration`** and verify the new cases fail.
- [ ] **Step 3: Add multipart parsing** with `MaxBytesReader`, bounded `ParseMultipartForm`, exactly `files[]`, optional `body`, and no reliance on client MIME.
- [ ] **Step 4: Add group-service orchestration** that rejects empty text plus no files, keeps ordinary JSON messages unchanged, and uses the existing idempotency/antispam flow.
- [ ] **Step 5: Add authorized download streaming** with `nosniff`, safe disposition, no filesystem path leakage, and group membership checks.
- [ ] **Step 6: Run focused and integration tests**; confirm message creation is atomic when one of multiple files fails.
- [ ] **Step 7: Commit** with `git add backend/internal/httpapi backend/internal/groups backend/internal/publicview backend/internal/integration backend/README.md && git commit -m "feat: send attachments with group messages"`.

### Task 6: Angular API contract and upload composer

**Files:**
- Modify: `web-client/src/app/core/api/models.ts` — add `MessageAttachment` and `GroupMessage.attachments`.
- Modify: `web-client/src/app/core/api/api.service.ts` — add `sendMessageWithFiles(groupID, body, files)` using `FormData` and upload progress.
- Create: `web-client/src/app/features/groups/attachment-upload.ts` (~180 lines) — client-side selection, count/size checks, and user-readable errors.
- Create: `web-client/src/app/features/groups/attachment-upload.spec.ts` (~180 lines) — selection and limit behavior.
- Modify: `web-client/src/app/features/groups/group.component.ts` — file picker state, caption/file-only submit, progress, cancellation/reset.
- Modify: `web-client/src/styles/screens/groups.css` — composer attachment controls and selected-file previews; keep the stylesheet below 300 lines.

**Interfaces:**
- `sendMessageWithFiles(groupID: string, body: string, files: File[]): Observable<UploadResult<GroupMessage>>`.
- `AttachmentUploadState` exposes selected files, total bytes, progress, and validation error without coupling to the component template.

- [ ] **Step 1: Write failing Vitest tests** for 3-file limit, 10 MiB media, 5 MiB archive candidate, caption-only and file-only state.
- [ ] **Step 2: Run `cd web-client && npm test -- --include='src/app/features/groups/attachment-upload.spec.ts'`** and confirm failure.
- [ ] **Step 3: Implement the pure upload-state helper** and client-side preflight checks; server remains authoritative.
- [ ] **Step 4: Add the API `FormData` method** and preserve JSON methods for ordinary messages/replies.
- [ ] **Step 5: Integrate the picker and caption into the group composer** with accessible labels, remove-file controls, progress, disabled duplicate submissions, and notification mapping.
- [ ] **Step 6: Run the focused client tests and `npm run lint:types`** and confirm they pass.
- [ ] **Step 7: Commit** with `git add web-client/src/app web-client/src/styles && git commit -m "feat: add attachment composer"`.

### Task 7: Render attachments and verify end-to-end behavior

**Files:**
- Create: `web-client/src/app/features/groups/message-attachments.component.ts` (~220 lines) — safe attachment card rendering, download links, image/video preview rules.
- Create: `web-client/src/app/features/groups/message-attachments.component.spec.ts` (~220 lines) — rendering and accessible link behavior.
- Modify: `web-client/src/app/features/groups/group-message-list.component.ts` — render the attachment component for sent messages and handle empty captions.
- Modify: `web-client/src/app/features/groups/group-message-list.component.spec.ts` — attachment rendering and file-only messages.
- Modify: `web-client/src/styles/screens/groups.css` — responsive attachment cards and media previews.
- Modify: `web-client/README.md` — user-visible upload rules and runtime `ffmpeg` note if relevant to deployment docs.

- [ ] **Step 1: Write failing component tests** for image, video, archive/unknown display, caption-only absence, and download URL rendering.
- [ ] **Step 2: Run `cd web-client && npm test -- --include='src/app/features/groups/message-attachments.component.spec.ts'`** and verify failure.
- [ ] **Step 3: Implement rendering** using server-provided MIME/kind and URL, never executing or injecting file content into HTML.
- [ ] **Step 4: Integrate with the message list** while preserving system messages, replies, delete actions, scroll behavior, and text-only messages.
- [ ] **Step 5: Run `cd web-client && npm test && npm run build`** and confirm all client checks pass.
- [ ] **Step 6: Commit** with `git add web-client && git commit -m "feat: render message attachments"`.

### Task 8: Runtime, documentation, limits and final verification

**Files:**
- Modify: `backend/Dockerfile` — install pinned Alpine `ffmpeg` runtime package.
- Modify: `backend/docker-compose.yml` — expose attachment storage/quota/TTL settings and mount storage under `/data`.
- Modify: `README.md` — mention attachment limits and `ffmpeg` requirement.
- Modify: `backend/CHANGELOG.md` — user-visible attachment feature entry.

- [ ] **Step 1: Add runtime configuration** with safe defaults and the explicit writable attachment directory `/data/attachments` under the existing data volume; the existing `/data/` ignore rule already excludes runtime files.
- [ ] **Step 2: Run `docker compose -f backend/docker-compose.yml config`** and confirm the service configuration is valid.
- [ ] **Step 3: Run backend checks:** `cd backend && gofmt -w . && go test ./... && go test -race ./... && go vet ./...`.
- [ ] **Step 4: Run client checks:** `cd web-client && npm test && npm run build && npm run lint:types`.
- [ ] **Step 5: Run repository checks:** `./scripts/verify-source-limits.sh` and `git diff --check`.
- [ ] **Step 6: Inspect source counts and line counts**; split any file approaching 220 lines before the hard 300-line ceiling and keep every directory at or below 5 source files.
- [ ] **Step 7: Run an end-to-end smoke test** with a valid image, caption-only attachment, misleading extension, archive traversal sample, expiry cleanup, and cross-group download denial.
- [ ] **Step 8: Commit** with `git add backend/Dockerfile backend/docker-compose.yml backend/.gitignore README.md backend/CHANGELOG.md && git commit -m "docs: document attachment deployment"`.

## Plan self-review

- Spec coverage: limits and multipart flow are covered by Tasks 1, 5, and 6; secure format handling by Task 3; metadata and atomicity by Task 2 and Task 5; TTL, quotas, and low-disk protection by Task 4; cleanup and deployment by Task 8; client rendering by Task 7.
- Placeholder scan: no implementation step depends on an unspecified format, file, command, or future decision.
- Type consistency: `attachments.Limits`, `ProcessedFile`, `storage.Service.Prepare`, `Batch.Commit/Rollback`, and `sendMessageWithFiles` are named consistently across tasks.
- Architectural limits: attachment processors are separated into `image`, `video`, `archive`, `storage`, and `cleanup` subdirectories; no planned source directory exceeds five files.
