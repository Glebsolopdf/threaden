# Account Blocks, Message Popover, and Attachment-Only Messages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Make temporary blocking account-scoped, anchor the message actions menu to its message, and allow attachment-only group messages end to end.

**Architecture:** Keep anonymous abuse protection IP-scoped, but resolve authenticated requests before temporary-ban checks and store their temporary state by account ID. Keep the message menu as a fixed overlay outside normal layout calculations, with one focused placement helper driven by the message anchor. Enforce the attachment-or-body rule in the group service and preserve empty-body-safe rendering at the UI/API boundary.

**Tech Stack:** Go 1.26+, SQLite store, chi HTTP API, Angular 21 standalone components, TypeScript, Vitest/jsdom.

## Global Constraints

- Source files must remain below 300 lines; ordinary source files should remain below 220–250 lines.
- No directory may contain more than 5 non-exempt source files.
- Preserve existing IP protection for anonymous traffic.
- Do not add a dependency when platform APIs and existing dependencies are sufficient.
- Do not modify unrelated attachment quota work already present in the working tree.
- Every behavior change starts with a failing regression test and is verified with fresh command output.

---

### Task 1: Account-scoped temporary blocks

**Files:**
- Create: backend/internal/store/rate/account_blocks.go — temporary account-block persistence and expiry lookup.
- Modify: backend/internal/store/tests/accountban_test.go — account isolation, expiry, and replacement tests.
- Modify: backend/internal/store/schema/migrations.go — add the account-ban table/index to the current schema migration sequence.
- Modify: backend/internal/store/schema/migration17.go — include the table in recovery/bootstrap SQL.
- Modify: backend/internal/httpapi/ban/enforcer.go — expose account-scoped check/record methods while keeping IP fallback.
- Modify: backend/internal/httpapi/router.go — make the abuse guard account-aware for authenticated requests.
- Modify: backend/internal/integration/avatar_test.go — HTTP regression for two accounts sharing one client IP.

**Interfaces:**
- Store.AccountBlockActive(ctx context.Context, userID string, now time.Time) (bool, time.Time, error).
- Store.SetAccountBlock(ctx context.Context, userID string, until time.Time) error.
- Enforcer.AccountBlocked(ctx context.Context, userID string) (bool, time.Time, error).
- Enforcer.NoteViolation(ctx context.Context, token, ip string) error remains the existing call-site contract.

- [x] Write store tests for user-a isolation from user-b, expiry, and same-user replacement.
- [ ] Run: cd backend && go test ./internal/store/tests -run TestAccountBan -count=1. Verify the failure is caused by missing account-ban methods/table.
- [ ] Add a separate user_id-keyed account_blocks table with an expiry index. Do not reuse the existing account_bans table, which stores historical maximum-level ban counts for account deletion. Use parameterized queries filtered by user_id; expired lookup must delete only that user row.
- [ ] Run the focused store tests and verify they pass.
- [ ] Add an integration test that triggers a temporary block for account A, asserts A gets 429 with the account-block code, asserts B succeeds from the same IP, then asserts A succeeds after expiry.
- [ ] Run: cd backend && go test ./internal/integration -run TestAccount -count=1. Verify the current IP-scoped behavior fails this regression.
- [ ] Resolve valid session identity in the abuse guard. For authenticated requests check only AccountBlocked(user.ID); for anonymous requests keep IPBanActive. When an authenticated rate-limit violation creates a temporary block, persist it for that user and set Retry-After from that record. Preserve existing anonymous IP escalation and existing account_bans cleanup/deletion accounting.
- [ ] Run: cd backend && go test ./internal/store/tests ./internal/httpapi/ban ./internal/integration -run Test -count=1.
- [ ] Commit with message: fix: scope temporary blocks to accounts, staging only Task 1 files.

### Task 2: Stable message action overlay positioning

**Files:**
- Create: web-client/src/app/features/groups/message-actions/message-actions-position.ts — pure viewport placement calculation.
- Create: web-client/src/app/features/groups/message-actions/message-actions-position.spec.ts — above/below/clamping tests.
- Modify: web-client/src/app/features/groups/message-actions/group-message-actions.component.ts — anchor-based positioning and lifecycle cleanup.
- Create: web-client/src/app/features/groups/message-actions/group-message-actions.component.spec.ts — component regression for anchor-derived coordinates.
- Modify: web-client/src/styles/screens/groups/chat.css — fixed overlay styling without flow-dependent offsets.

**Interfaces:**
- MessageRect is the width/height/top/bottom/left/right subset of DOMRect.
- MenuPlacement is { top: number; left: number; above: boolean }.
- placeMessageMenu(anchor, menu, viewport, gap): MenuPlacement.

- [ ] Write pure placement tests for preferred-above, below fallback, horizontal clamping, and menus taller than the viewport.
- [ ] Run: cd web-client && npx vitest run src/app/features/groups/message-actions-position.spec.ts. Verify the expected missing-function failure.
- [x] Implement placeMessageMenu using the anchor rectangle, an 8px inset, a gap, above-first placement, below fallback, and viewport clamps. Never use pointer coordinates for placement.
- [ ] Run the placement tests and verify they pass.
- [ ] Write a component test where pointer coordinates differ from the message bubble rectangle; assert the menu uses the bubble rectangle. Assert scroll/resize/ResizeObserver cleanup on close and destroy.
- [ ] Run the component test and verify it fails against the current clientX/clientY implementation.
- [ ] Use the existing message bubble as anchor, measure after mount, recalculate in requestAnimationFrame, and install capture-phase scroll, window resize, and ResizeObserver updates while open. Remove all listeners, observer handles, timers, and pending frames on close/destroy. Keep long-press and swipe-to-reply behavior unchanged.
- [ ] Run: cd web-client && npx vitest run src/app/features/groups/message-actions-position.spec.ts src/app/features/groups/group-message-actions.component.spec.ts src/app/features/groups/group-message-list.component.spec.ts.
- [ ] Commit with message: fix: anchor message actions to chat bubbles, staging only Task 2 files.

### Task 3: Attachment-only messages

**Files:**
- Modify: backend/internal/groups/messages.go — enforce the attachment-or-body rule in the domain service.
- Create: backend/internal/groups/tests/message_content_test.go — service rule cases.
- Modify: backend/internal/integration/groups_test.go — multipart API and history coverage.
- Modify: web-client/src/app/features/groups/attachments/message-composer.component.ts — shared send predicate and guard.
- Modify: web-client/src/app/features/groups/attachments/attachment-upload.spec.ts — text/file combinations.
- Modify: web-client/src/app/features/groups/group-message-list.component.ts — empty-body-safe history/reply rendering.
- Modify: web-client/src/app/features/groups/group-message-list.component.spec.ts — attachment-only rendering regression.
- Modify: web-client/src/app/features/groups/attachments/message-attachments.component.ts only if the regression identifies a body assumption.

**Interfaces:**
- canSendMessage(body: string, attachmentCount: number): boolean returns true exactly when body.trim().length > 0 or attachmentCount > 0.
- SendWithAttachments accepts an empty trimmed body when the committed batch contains at least one attachment.

- [x] Add backend content-rule cases for text-only, text-plus-attachment, attachment-only, whitespace-plus-attachment, and empty-without-attachment.
- [ ] Run: cd backend && go test ./internal/groups ./internal/integration -run Test -count=1. Verify the expected failure.
- [ ] Keep Send strict for text-only messages. In SendWithAttachments reject only when both trimmed body and attachment count are empty; preserve trim and rune-limit validation. Keep omitted multipart body as empty string.
- [ ] Run the backend attachment tests and verify they pass.
- [ ] Add frontend tests for hello/none, hello/file, empty/file, whitespace/file, and empty/none. Render an attachment-only history item and assert attachment markup exists without requiring an empty paragraph or body-dependent exception.
- [ ] Run: cd web-client && npx vitest run src/app/features/groups/attachments/message-composer.component.spec.ts src/app/features/groups/group-message-list.component.spec.ts. Verify the expected failure before implementation.
- [ ] Use canSendMessage for the disabled state and send guard, keep typing notifications text-only, and keep conditional body rendering. Make reply/summary access tolerate an empty body without changing unrelated UX.
- [ ] Run focused frontend tests and then cd backend && go test ./internal/... -count=1.
- [ ] Commit with message: fix: allow messages with attachments and no text, staging only Task 3 files.

### Task 4: Full verification and structural review

- [x] Run cd backend && gofmt on changed Go files; Go tests remain blocked by the installed Go 1.22.2 versus go.mod's Go 1.26 requirement.
- [x] Run cd web-client && npm run lint:types and npm test.
- [x] Run ./scripts/verify-source-limits.sh.
- [x] Run git diff --check, inspect git diff --stat, and confirm unrelated dirty files are not staged.
- [ ] Record maximum source-file line count, directories at the 5-file limit, warnings, and any pre-existing failures. Do not claim checks passed without fresh output.
