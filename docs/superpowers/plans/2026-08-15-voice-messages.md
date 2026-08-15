# Голосовые сообщения Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить загрузку аудиофайлов и запись голосовых сообщений до 5 минут, сохранив общую квоту и TTL вложений, а также закрепить полную блокировку запросов временно заблокированного аккаунта.

**Architecture:** Аудио расширяет существующий домен `attachments` новым видом `audio` и проходит текущий `multipart → processor → storage → cleanup` pipeline. Запись браузера живёт в отдельном feature-модуле и отправляет полученный Blob через существующий multipart API. Проверка account block остаётся на общем HTTP-уровне до маршрутизации; интеграционные тесты докажут, что public/protected, cookie/Bearer и multipart-маршруты не обходят её.

**Tech Stack:** Go 1.26, SQLite, `net/http`, существующий attachment storage, Angular 21, TypeScript, browser `MediaRecorder`, Vitest.

## Global Constraints

- Исходные файлы не превышают 300 строк; обычный рабочий предел — 220–250 строк.
- В одном каталоге не более 5 исходных файлов.
- Аудио считается обычным вложением: общая активная/суточная/глобальная квота, максимум 3 вложения и текущий retention 72 часа.
- Запись ограничена 5 минутами и автоматически останавливается по достижении лимита.
- Поддерживаются WebM/Opus, OGG/Opus, MP3, WAV и M4A/MP4; формат проверяется по содержимому и MIME.
- Новые зависимости не добавляются.
- Активная блокировка отклоняет каждый запрос с cookie/Bearer-токеном аккаунта, включая logout; анонимные и незаблокированные аккаунты работают как раньше.

---

### Task 1: Backend audio domain and format detection

**Files:**
- Modify: `backend/internal/attachments/limits.go` — добавить `KindAudio`.
- Modify: `backend/internal/attachments/detect.go` — распознавать согласованные аудиосигнатуры и MIME.
- Modify: `backend/internal/attachments/processor.go` — передавать проверенное аудио в обычное файловое хранилище без перекодирования.
- Create: `backend/internal/attachments/audio/audio.go` — компактные проверки контейнерных сигнатур, если они не помещаются в detector без смешения ответственностей.
- Test: `backend/internal/attachments/detect_test.go` и `backend/internal/attachments/processor_test.go` (создать processor test рядом с текущими тестами, если его нет).

**Interfaces:**
- Consumes: `attachments.Detect`, `attachments.Processor.Process`.
- Produces: `KindAudio`, detection result `audio`, `ProcessedFile{Kind: KindAudio}`; downstream storage uses these without a new API.

- [ ] **Step 1: Write failing detector tests**

Добавить минимальные тесты на валидные сигнатуры/заголовки WebM, OGG, MP3, WAV и M4A/MP4, проверяя `kind == "audio"` и ожидаемый MIME; добавить тест, что текстовый файл с аудио-расширением отклоняется.

- [ ] **Step 2: Run detector tests to verify failure**

Run: `cd backend && go test ./internal/attachments -run 'TestDetect.*Audio|TestDetectRejectsFakeAudio' -count=1`

Expected: FAIL because `KindAudio` and audio detection are not implemented.

- [ ] **Step 3: Implement minimal audio detection**

Распознавать только реальные контейнерные признаки: `RIFF....WAVE`, `OggS`, ID3/MP3 frame sync, ISO-BMFF `ftyp` с audio-подходящими brands; WebM/Matroska — по EBML и audio MIME/имени только в сочетании с содержимым. Не принимать расширение само по себе. Не менять существующие image/video/archive/file ветки.

- [ ] **Step 4: Write failing processor test**

Создать тест с временным валидным WAV/OGG-образцом, вызвать `Processor.Process`, проверить `KindAudio`, MIME, размер и существование временного output path.

- [ ] **Step 5: Run processor test to verify failure**

Run: `cd backend && go test ./internal/attachments -run TestProcessorProcessesAudio -count=1`

Expected: FAIL because processor has no audio branch.

- [ ] **Step 6: Implement passthrough processor branch**

Добавить аудио-ветку, копирующую input во временный `threaden-audio-*` файл и возвращающую `KindAudio`; ограничение входного размера и cleanup temporary files должны использовать существующий код.

- [ ] **Step 7: Run focused backend tests**

Run: `cd backend && go test ./internal/attachments -count=1`

Expected: PASS with existing and new detector/processor tests.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/attachments
git commit -m "feat: support audio attachments"
```

### Task 2: Backend attachment API, public model, and shared quota/TTL regression

**Files:**
- Modify: `backend/internal/model/model.go` — документировать/расширить attachment kind через фактический `KindAudio` без нового storage поля.
- Modify: `backend/internal/publicview/publicview.go` — пропустить `audio` как обычное публичное вложение.
- Modify: `backend/internal/attachments/storage/files.go` — убедиться, что commit сохраняет audio metadata тем же `Retention` и reservation.
- Test: `backend/internal/attachments/storage/files_test.go` — quota/commit regression для audio.
- Test: `backend/internal/store/tests/attachments/deletion_test.go` — TTL/cleanup regression для audio metadata.
- Test: `backend/internal/integration/attachments_test.go` or existing attachment integration test file — multipart audio send and public response.

**Interfaces:**
- Consumes: `ProcessedFile.KindAudio`, existing `storage.Batch.Commit`.
- Produces: public `MessageAttachment.kind == "audio"`, same quota sums and `ExpiresAt` behavior.

- [ ] **Step 1: Write failing storage and integration tests**

Проверить, что аудио-processed file занимает bytes в `SumForOwner`/`SumAll`, получает `ExpiresAt = CreatedAt + limits.Retention`, возвращается как `kind: audio`, и multipart message with audio is accepted. Добавить expiry test, где audio and image are both returned by `DeleteExpired`.

- [ ] **Step 2: Run tests to verify failure**

Run: `cd backend && go test ./internal/attachments/storage ./internal/store/tests ./internal/integration -run 'Audio|Attachment' -count=1`

Expected: FAIL until audio processing and public mapping are connected.

- [ ] **Step 3: Implement only required mappings**

Не добавлять отдельную квоту, таблицу, cleanup-ветку или endpoint: существующий `storage.Service`, metadata store, `cleanup.Service` и message deletion должны работать без специальных условий для `audio`.

- [ ] **Step 4: Run focused tests**

Run: `cd backend && go test ./internal/attachments/... ./internal/store/tests ./internal/integration -run 'Audio|Attachment' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/model backend/internal/publicview backend/internal/attachments backend/internal/store/tests backend/internal/integration
git commit -m "test: keep audio in attachment quota and retention"
```

### Task 3: Browser audio selection and audio rendering

**Files:**
- Modify: `web-client/src/app/core/api/models.ts` — добавить `audio` в `MessageAttachment.kind`.
- Modify: `web-client/src/app/features/groups/attachments/attachment-upload.ts` — классификация audio и тот же input-size validation.
- Modify: `web-client/src/app/features/groups/attachments/message-attachments.component.ts` — `<audio controls>` с безопасной fallback-ссылкой.
- Modify: `web-client/src/app/features/groups/attachments/message-composer.component.ts` — подключить audio selection и voice recorder control without storing recorder lifecycle in template logic.
- Create: `web-client/src/app/features/groups/attachments/voice/voice-recorder.ts` — MediaRecorder state machine and Blob result, under 220 lines.
- Test: `web-client/src/app/features/groups/attachments/attachment-upload.spec.ts` and `voice/voice-recorder.spec.ts`.

**Interfaces:**
- Consumes: browser `MediaRecorder`, `Blob`, `File`.
- Produces: `VoiceRecorder` with `start(): Promise<void>`, `stop(): Promise<Blob>`, `cancel(): void`, `state`/elapsed signals or equivalent focused observable state; composer converts Blob to `File` and reuses `sendMessageWithFiles`.

- [ ] **Step 1: Write failing upload/render tests**

Проверить `attachmentKind` для `audio/*` и известных расширений, лимит количества/размера, а также что `MessageAttachmentsComponent` отдаёт audio attachment через audio player path.

- [ ] **Step 2: Run web tests to verify failure**

Run: `cd web-client && npm test -- --run src/app/features/groups/attachments/attachment-upload.spec.ts`

Expected: FAIL because audio is typed/classified as generic file and model/template has no audio branch.

- [ ] **Step 3: Implement audio selection and rendering**

Добавить `audio` в union, включить audio MIME/extension classification and render `<audio controls preload="metadata">`; keep existing image/video/file branches unchanged.

- [ ] **Step 4: Write failing recorder tests**

Подменить только browser boundary `navigator.mediaDevices.getUserMedia` и `MediaRecorder` тестовым fake, затем проверить start/stop, cancel, unsupported recorder, permission rejection, and automatic stop callback at 5 minutes using fake timers.

- [ ] **Step 5: Run recorder tests to verify failure**

Run: `cd web-client && npm test -- --run src/app/features/groups/attachments/voice/voice-recorder.spec.ts`

Expected: FAIL because recorder module does not exist.

- [ ] **Step 6: Implement recorder state machine**

Выбрать первый поддерживаемый MIME из `audio/webm;codecs=opus`, `audio/ogg;codecs=opus`, `audio/mp4`, `audio/wav`; освобождать MediaStream tracks on stop/cancel/error/destroy; enforce `MAX_RECORDING_MS = 5 * 60 * 1000`.

- [ ] **Step 7: Connect composer**

Добавить start/stop/cancel UI, elapsed timer, disabled send state while recording, automatic conversion of resulting Blob to a named `.webm`/appropriate extension File, and reuse existing multipart send path. Display microphone errors through existing `NotificationStore`.

- [ ] **Step 8: Run web focused tests**

Run: `cd web-client && npm test -- --run src/app/features/groups/attachments`

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add web-client/src/app/core/api/models.ts web-client/src/app/features/groups/attachments
git commit -m "feat: record and render voice messages"
```

### Task 4: Account block request coverage

**Files:**
- Modify: `backend/internal/integration/avatar_test.go` or create `backend/internal/integration/account_block_test.go` — account-wide request matrix.
- Modify: `backend/internal/httpapi/router.go` only if a test exposes a route outside the common `abuseGuard`; preserve existing `account_blocked`/`Retry-After` response.
- Modify: `web-client/src/app/core/auth/auth.guard.ts` only if the existing 429 handling fails to route blocked responses; do not change unrelated auth behavior.

**Interfaces:**
- Consumes: `store.SetAccountBlock`, `httpapi.NewWithSecurity`, existing test API helpers.
- Produces: regression proof that active block is enforced before public/protected route handlers and multipart body processing.

- [ ] **Step 1: Write failing integration matrix**

Заблокировать account A and assert HTTP 429 + `account_blocked` for `GET /v1/me`, `GET /v1/groups/{id}/messages`, `PATCH /v1/me`, multipart `POST /v1/groups/{id}/messages`, and `DELETE /v1/auth/logout`; assert account B and anonymous public group read remain successful; assert A succeeds after expiry.

- [ ] **Step 2: Run matrix to verify behavior**

Run: `cd backend && go test ./internal/integration -run TestTemporaryBlockRejectsAllAuthenticatedRequests -count=1`

Expected: If current middleware is complete, PASS immediately; record that no production change is needed. If any route bypasses the guard, the test must fail with the specific route.

- [ ] **Step 3: Fix only exposed bypasses**

Keep block enforcement before route dispatch and before multipart parsing. Do not special-case individual endpoints; ensure the check identifies both cookie and Bearer sessions and retains unaffected anonymous/unblocked behavior.

- [ ] **Step 4: Re-run block tests**

Run: `cd backend && go test ./internal/integration -run 'TemporaryBlock|Ban' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit if production/test changes are new**

```bash
git add backend/internal/integration backend/internal/httpapi web-client/src/app/core/auth
git commit -m "test: enforce account-wide temporary blocks"
```

### Task 5: Documentation, limits, and full verification

**Files:**
- Modify: `README.md` — mention audio messages and unchanged 72-hour retention/quotas.
- Modify: `backend/README.md` — document accepted audio formats and 5-minute browser recording.
- Modify: `web-client/README.md` only if client-specific behavior is documented there.

- [ ] **Step 1: Update user-facing documentation**

Описать загрузку готовых аудиофайлов, запись до 5 минут, общую квоту и удаление через 72 часа; отдельно не обещать поддержку форматов, которые detector не принимает.

- [ ] **Step 2: Run formatting**

Run: `gofmt -w backend` and `cd web-client && npx prettier --check src` only if Prettier is configured; otherwise use the repository’s existing formatter command.

- [ ] **Step 3: Run backend checks**

Run: `cd backend && go test ./... && go test -race ./... && go vet ./...`

Expected: all commands exit 0; report any unavailable `govulncheck` separately rather than claiming it ran.

- [ ] **Step 4: Run web checks**

Run: `cd web-client && npm test -- --run && npm run build`

Expected: tests and production build exit 0.

- [ ] **Step 5: Verify architecture limits**

Run: `./scripts/verify-source-limits.sh`

Then inspect `find`/`wc -l` output to confirm no source file exceeds 300 lines and no non-exempt directory exceeds 5 source files; report the maximum line count and any directory at exactly 5 files.

- [ ] **Step 6: Review final diff**

Run: `git status --short && git diff --stat HEAD~4..HEAD` and inspect all changed files for unrelated edits, dead code, unfinished TODOs, and accidental changes to the user’s pre-existing dirty worktree.
