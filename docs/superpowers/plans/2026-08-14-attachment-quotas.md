# Квоты и отложенное удаление вложений — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task with verification checkpoints.

**Goal:** Добавить серверный раздел «Квоты» в аккаунт с отображением лимитов и безопасным удалением сообщений с вложениями через 5 минут с возможностью отмены.

**Architecture:** Backend будет отдавать единый quota response из конфигурации и базы. Отложенное удаление хранится в SQLite с уникальным активным запросом на пользователя и выполняется существующим cleanup-циклом. Angular добавит отдельный account tab после профиля и вызывает только защищённые endpoint текущего пользователя.

**Tech Stack:** Go, Chi, SQLite migrations, Angular 21, RxJS, Vitest, существующий `threadenctl`.

## Global Constraints

- Исходные файлы не более 300 строк; обычный рабочий размер — менее 250 строк.
- Директории содержат не более 5 исходных файлов; для новых boundary использовать осмысленные поддиректории.
- Удаление применяется только к сообщениям пользователя, содержащим его вложения.
- Запрос удаления переживает перезагрузку и перезапуск backend.
- Отмена разрешена только до `execute_at`; повторная постановка не создаёт второй активный запрос.
- Клиент не передаёт `user_id`; backend получает его из текущей сессии.
- Не добавлять новые зависимости и не коммитить без отдельного указания пользователя.

---

### Task 1: Persistence model for scheduled attachment deletion

**Files:**
- Modify: `backend/internal/store/schema/migration17.go` — добавить `migration22` с таблицей `attachment_delete_requests`, уникальным активным запросом на пользователя и индексами по `execute_at`.
- Modify: `backend/internal/store/schema/migrations.go` — поднять `LatestVersion` до 22 и подключить migration 22 в существующий порядок миграций.
- Create: `backend/internal/store/attachments/deletion.go` — операции create/get/cancel/claim/delete для запроса и выборка пользовательских сообщений с вложениями.
- Create: `backend/internal/store/tests/attachments/deletion_test.go` — SQL-поведение активного запроса, повторной постановки и отмены.

**Interfaces:**
- `CreateDeleteRequest(ctx, db, userID string, createdAt, executeAt time.Time) (model.AttachmentDeleteRequest, error)` возвращает существующий pending request при конфликте уникальности.
- `GetDeleteRequest(ctx, db, userID string) (model.AttachmentDeleteRequest, error)` возвращает `sql.ErrNoRows`, если запроса нет.
- `CancelDeleteRequest(ctx, db, userID string) error` отменяет только pending request.
- `ClaimDueDeleteRequests(ctx, db, now time.Time, limit int) ([]model.AttachmentDeleteRequest, error)` выбирает due-запросы ограниченной batch-порцией.
- `DeleteUserAttachmentMessages(ctx, db, userID string, requestID string) ([]model.Attachment, error)` в транзакции удаляет сообщения пользователя с attachment rows и возвращает физические файлы для удаления.

- [ ] **Step 1: Add the failing persistence test**

  Create a SQLite test database through the established migration helper. Assert that the first request is stored, a second request for the same user returns the same pending request, cancellation removes it, and a different user remains isolated.

- [ ] **Step 2: Run the focused test and verify RED**

  Run: `cd backend && /tmp/threaden-go126/bin/go test ./internal/store/tests/attachments -run 'TestDeleteRequest' -count=1`

  Expected: compilation/test failure because migration, model, and store operations do not exist yet.

- [ ] **Step 3: Add the model and migration**

  Add `model.AttachmentDeleteRequest` with `ID`, `UserID`, `CreatedAt`, and `ExecuteAt`. Add a table with a random public-safe ID, foreign key to users, unique partial index for pending requests, and an index for due execution.

- [ ] **Step 4: Implement store operations**

  Keep SQL in `store/attachments/deletion.go`. Use a transaction for deleting message rows and attachment metadata, deduplicate message IDs, and return paths before metadata deletion so the cleanup layer can remove files without trusting client input.

- [ ] **Step 5: Run the focused test and verify GREEN**

  Run the same focused command. Expected: PASS, including migration and ownership isolation assertions.

---

### Task 2: Quota and deletion application service

**Files:**
- Create: `backend/internal/attachments/account/service.go` — quota snapshot, schedule, cancel, and cleanup use cases.
- Create: `backend/internal/attachments/account/service_test.go` — service behavior and boundary conditions.
- Modify: `backend/internal/attachments/cleanup/cleanup.go` — execute due deletion requests before/alongside retention cleanup and remove returned files safely.
- Modify: `backend/cmd/api/main.go` — construct the account attachment service and provide it to HTTP and cleanup composition.

**Interfaces:**
- `Service.Quotas(ctx, userID string) (QuotaSnapshot, error)` combines configured limits with `SumForOwner` and `SumCreatedSince`.
- `Service.ScheduleDeleteAll(ctx, userID string, now time.Time) (DeleteRequest, error)` schedules exactly `now + 5*time.Minute`.
- `Service.CancelDeleteAll(ctx, userID string) error` returns a domain error when no pending request exists or execution has started.
- `Service.RunDueDeletes(ctx, now time.Time, batchSize int) error` claims requests, deletes owned message/attachment metadata, removes only returned paths, and logs filesystem failures.

- [ ] **Step 1: Write failing service tests**

  Cover quota values from configuration, current stored/daily usage, fixed 5-minute execution time, idempotent schedule, cancellation before due time, and exclusion of another user’s attachments.

- [ ] **Step 2: Run service tests and verify RED**

  Run: `cd backend && /tmp/threaden-go126/bin/go test ./internal/attachments/account -run 'Test' -count=1`

  Expected: package/types are missing and tests fail for the intended reason.

- [ ] **Step 3: Implement quota and lifecycle service**

  Use the existing `attachments.Limits` as the single source for displayed limits. Keep the 30-minute grace period in the service, not in the HTTP handler. Make deletion idempotent when files or metadata were already removed.

- [ ] **Step 4: Integrate with the cleanup loop**

  Run due user-delete requests in the existing minute loop with the same bounded batch discipline as expiry cleanup. Do not delete a message merely because it has an attachment from another owner.

- [ ] **Step 5: Run service and full backend tests**

  Run: `cd backend && /tmp/threaden-go126/bin/go test ./... -count=1 -timeout=120s`

  Expected: PASS.

---

### Task 3: Protected quota HTTP API

**Files:**
- Create: `backend/internal/httpapi/account/handler.go` — quota response and schedule/cancel handlers.
- Create: `backend/internal/httpapi/account/handler_test.go` — auth, idempotency, cancellation, and response tests.
- Modify: `backend/internal/httpapi/router.go` — mount `/v1/account/quotas` and `/v1/account/attachments/delete-all` under the existing protected router.

**Interfaces:**
- `GET /v1/account/quotas` → `{usage, limits, pending_delete}`.
- `POST /v1/account/attachments/delete-all` → `202` or `200` with `{execute_at}`.
- `DELETE /v1/account/attachments/delete-all` → `204` on cancellation.

- [ ] **Step 1: Add failing HTTP tests**

  Assert unauthenticated requests are rejected, authenticated requests cannot select another user, repeated POST returns one schedule, DELETE cancels it, and DELETE after execution returns a conflict error.

- [ ] **Step 2: Run focused HTTP tests and verify RED**

  Run: `cd backend && /tmp/threaden-go126/bin/go test ./internal/httpapi/account -run 'Test' -count=1`

  Expected: failure because the handler package and routes are not present.

- [ ] **Step 3: Implement handlers and route wiring**

  Map domain errors to stable API codes, return UTC timestamps, and never expose filesystem paths or SQL errors. Use the current-user hook from the protected router.

- [ ] **Step 4: Run HTTP tests and full backend tests**

  Run: `cd backend && /tmp/threaden-go126/bin/go test ./internal/httpapi/account ./... -count=1 -timeout=120s`

  Expected: PASS.

---

### Task 4: Angular quota API models and account tab

**Files:**
- Modify: `web-client/src/app/core/api/models.ts` — quota, usage, limit, and pending deletion types.
- Modify: `web-client/src/app/core/api/api.service.ts` — quota GET, schedule POST, cancel DELETE methods.
- Modify: `web-client/src/app/features/account/account-dialog.component.ts` — add `quotas` tab after profile, load state, confirmation, schedule/cancel actions, and accessible status copy.
- Modify: `web-client/src/styles/screens/settings/account-dialog.css` — quota cards, usage rows, danger-zone styling, and mobile layout.
- Create: `web-client/src/app/features/account/quota-view.ts` — pure formatting/state helpers if the dialog approaches the file-size limit.

**Interfaces:**
- `ApiService.quotas(): Observable<AccountQuotas>`
- `ApiService.scheduleAttachmentDeletion(): Observable<PendingAttachmentDeletion>`
- `ApiService.cancelAttachmentDeletion(): Observable<void>`

- [ ] **Step 1: Add failing frontend tests**

  Test quota byte formatting, pending-delete label states, and that the tab is ordered after profile. Add an interaction test for schedule then cancel using the existing Angular/Vitest conventions.

- [ ] **Step 2: Run focused frontend tests and verify RED**

  Run: `cd web-client && npm test -- --watch=false`

  Expected: failures for missing quota types, methods, and tab state.

- [ ] **Step 3: Implement API types and methods**

  Keep endpoint paths centralized in `ApiService`; map timestamps as ISO strings and preserve the backend response instead of duplicating configured numeric limits in the browser.

- [ ] **Step 4: Implement the quota tab**

  Place a compact “usage” card first, a two-column limits list second, and the destructive action in a clearly separated danger zone at the bottom. The confirmation dialog must explain that entire messages with attachments and their captions will be removed after 30 minutes. Disable buttons while requests are pending and respect reduced-motion/mobile scrolling.

- [ ] **Step 5: Run frontend verification**

  Run: `cd web-client && npm test -- --watch=false && npm run lint:types && npm run build`

  Expected: PASS; the existing initial bundle budget warning may remain and must be reported, not hidden.

---

### Task 5: Documentation, limits, and deployment verification

**Files:**
- Modify: `README.md` or `backend/README.md` — document quota endpoint behavior and the 30-minute deletion grace period if the project docs expose user-facing API behavior.
- Modify: `CHANGELOG.md` only if the repository already maintains one and release policy requires it.

- [ ] **Step 1: Run complete checks**

  Run backend tests, frontend tests, type-check, production build, `git diff --check`, and the repository-specific source-file/directory limit inspection.

- [ ] **Step 2: Review ownership and destructive-action paths**

  Confirm the database query scopes by authenticated user, cancellation cannot affect another request, cleanup removes only returned paths, and the UI cannot call endpoints without a session.

- [ ] **Step 3: Deploy only after verification**

  Run `./threadenctl.sh restart --full --backend` and `./threadenctl.sh restart --full --public` only if the user requests deployment in this implementation turn. Verify backend `/healthz` and public HTTP 200.
