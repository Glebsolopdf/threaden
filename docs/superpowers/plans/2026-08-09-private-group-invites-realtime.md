# Private Group Invites and Realtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make private-group membership realtime reliable, show the history notice once after joining, and route every invite flow through a safe invitation screen.

**Architecture:** Keep the existing API and SSE authorization boundary. Add a focused invite feature directory for token parsing and invitation presentation, keep group chat orchestration in `GroupsStore`/`GroupComponent`, and keep invite-link entry in the shell. Use same-origin URL parsing only; never navigate to or fetch an external URL.

**Tech Stack:** Angular 21 standalone components, Signals, Reactive Forms, RxJS, Vitest, existing Go API/SSE.

## Global Constraints

- No source file may exceed 300 lines; ordinary files should remain below 220–250 lines.
- No non-exempt directory may contain more than 5 source files.
- Preserve the existing untracked `design-previews/` directory.
- Do not change backend contracts unless a failing realtime regression proves the current boundary is insufficient.
- External invite URLs must be rejected before any network request.

---

### Task 1: Reproduce and lock down private-member realtime behavior

**Files:**
- Modify: `backend/internal/integration/groups_test.go` — add private-member SSE coverage if the current server behavior is not already covered.
- Create: `web-client/src/app/core/events/typing.store.spec.ts` — test group-scoped typing state and expiry.
- Modify: `web-client/src/app/core/events/groups.store.spec.ts` — add a private-group message merge regression.

**Interfaces:**
- Consume existing `GroupsStore.mergeMessage(message)` and `TypingStore.update(groupID, event)` APIs.
- Produce tests that prove events for the current private group are applied and events for another group are ignored.

- [ ] **Step 1: Add the failing client regression tests**

  Set a current group with `visibility: 'private'`, merge a message with that group id, and assert it appears. Merge a message for another group and assert it does not appear. For typing, update one group with an active member and assert `labelFor` returns the label only for that group.

- [ ] **Step 2: Run the focused tests and verify the failure**

  Run `npm test -- --include='src/app/core/events/groups.store.spec.ts'` and the typing spec command supported by the Angular test runner. The failure must be caused by the missing private-group behavior, not test setup.

- [ ] **Step 3: Add the smallest client-side fix**

  Trace the failing boundary through `EventStreamService`, `GroupsStore`, and `TypingStore`. Preserve group-id filtering and membership privacy. If current code already passes, keep production code unchanged and use the regression tests as the proof that the bug is not in this boundary.

- [ ] **Step 4: Run focused and backend tests**

  Run `npm test` in `web-client` and `go test ./internal/integration/...` in `backend`; both must pass before moving to UI work.

### Task 2: Add safe invite-token parsing and invitation presentation

**Files:**
- Create: `web-client/src/app/features/groups/invite/invite-token.ts` — pure parser for raw token, `/invite/<token>`, and same-origin absolute URLs.
- Create: `web-client/src/app/features/groups/invite/invite-token.spec.ts` — parser tests for accepted and rejected inputs.
- Create: `web-client/src/app/features/groups/invite/group-invite.component.ts` — standalone invitation screen with avatar, title, copy, accept, and cancel actions.
- Modify: `web-client/src/app/features/groups/group.component.ts` — render the invite component for `inviteToken` and keep chat markup for group-id routes.
- Modify: `web-client/src/app/app.routes.ts` — retain the existing invite route and title, changing only the component composition if needed.

**Interfaces:**
- `parseInviteToken(value: string, origin: string): string | null` accepts only a non-empty invite token, a relative `/invite/<token>` path, or an absolute URL whose origin equals `origin`.
- `GroupInviteComponent` consumes `GroupInfo`, `inviteToken`, and emits `accepted`/`cancelled` actions through callbacks or outputs.
- `GroupsStore.joinCurrent(inviteToken)` remains the only join operation.

- [ ] **Step 1: Write parser tests first**

  Cover a raw token, `/invite/inv_123`, `https://current.example/invite/inv_123`, `https://evil.example/invite/inv_123`, `javascript:...`, `data:...`, a file path, a wrong route, and empty input.

- [ ] **Step 2: Run parser tests and verify they fail**

  Run `npm test -- --include='src/app/features/groups/invite/invite-token.spec.ts'`; confirm the failure is that the parser is not implemented.

- [ ] **Step 3: Implement the parser and invitation component**

  Use `URL` only for parsing, compare `url.origin` with `location.origin`, require pathname segments exactly matching `invite/<token>`, and return the final token without performing navigation or HTTP requests. The component must call a supplied accept action and route cancellation to `/`.

- [ ] **Step 4: Connect the invite route state**

  When `inviteToken` is present, load the existing `GET /v1/invites/{token}` preview, render only the invitation screen, and do not render messages, composer, group menu, or join dock. On accept, call the existing join method and navigate to `/groups/:id`.

- [ ] **Step 5: Run the invite tests and build**

  Run the focused parser tests and `npm run build` in `web-client`.

### Task 3: Add invite-link entry to the new-group dialog

**Files:**
- Create: `web-client/src/app/features/shell/invite-link.ts` — shell-level adapter that parses a submitted value and returns a safe token or a validation error.
- Create: `web-client/src/app/features/shell/invite-link.spec.ts` — tests that the shell accepts raw/same-origin invite values and rejects external URLs.
- Modify: `web-client/src/app/features/shell/shell.component.ts` — add the invite section, form control, loading state, validation, and navigation to `/invite/:token`.
- Modify: `web-client/src/styles/screens/groups/dialogs.css` — style the new invite section using existing dialog tokens and responsive behavior.

**Interfaces:**
- `parseInviteInput(value: string): string | null` delegates to the safe invite parser without network access.
- Shell submission navigates to `/invite/:token`; it never calls `window.open`, `fetch`, or an external URL.

- [ ] **Step 1: Add failing shell parser tests**

  Assert raw and same-origin values return the token, while external origins and unsupported schemes return `null`.

- [ ] **Step 2: Verify the tests fail**

  Run the focused shell invite spec and confirm the missing parser failure.

- [ ] **Step 3: Implement the dialog flow**

  Add the copy `У вас есть ссылка приглашения?`, input placeholder `Вставьте её сюда`, and button `Присоединиться`. Invalid input stays in the dialog and uses the notification store; valid input closes the dialog and navigates to the invitation screen.

- [ ] **Step 4: Style and verify**

  Keep the new section below the create-group controls, reuse `.dialog-card`/`.temporary-section` patterns, and run the shell tests plus `npm run build`.

### Task 4: Show the history notice once after joining

**Files:**
- Create: `web-client/src/app/features/groups/history/history-notice.ts` — session-scoped one-time display state keyed by group id.
- Create: `web-client/src/app/features/groups/history/history-notice.spec.ts` — tests for first display, repeated display suppression, and ten-second expiry.
- Modify: `web-client/src/app/features/groups/group.component.ts` — show the private history banner only when the one-time state is active and schedule its removal after 10 seconds.
- Modify: `web-client/src/styles/screens/groups/chat.css` — add the existing leave animation/class if the current styles do not already support the disappearing notice.

**Interfaces:**
- `HistoryNoticeState` exposes `showAfterJoin(groupID): boolean` and `isVisible(groupID): boolean` or an equivalent minimal API.
- Storage failures fall back to in-memory state and must not prevent the group from opening.

- [ ] **Step 1: Write the one-time state tests**

  Assert the first post-join call returns visible, the second call returns hidden, a different group is independent, and the visible state expires after 10 seconds using fake timers.

- [ ] **Step 2: Verify the tests fail**

  Run the focused history-notice spec and confirm the state module is missing.

- [ ] **Step 3: Implement state and connect it to successful joining**

  Mark the group only after `joinInvite`/`joinGroup` succeeds. Store the displayed marker in `sessionStorage`; use a signal/timer for the current component view and remove the banner after 10 seconds. Do not show it for ordinary group reloads.

- [ ] **Step 4: Verify behavior**

  Run the focused history spec, all web-client tests, and `npm run build`.

### Task 5: Final integration and structural verification

**Files:**
- Modify only files from Tasks 1–4 if integration fixes are required.
- Do not modify `design-previews/`.

- [ ] **Step 1: Run all checks**

  Run `npm test` and `npm run build` in `web-client`, `go test ./...` in `backend`, and `git diff --check`.

- [ ] **Step 2: Verify source limits**

  Excluding `.git`, `node_modules`, and build output, measure all source files and directories. Confirm maximum file length is at most 300 lines and no directory has more than 5 source files.

- [ ] **Step 3: Review the final behavior and diff**

  Confirm external invite links make no requests, invite accept is explicit, cancel does not join, the history notice appears once after joining, and private member messages/typing remain group-scoped.
