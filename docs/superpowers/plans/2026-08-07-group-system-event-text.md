# Group System Event Text Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move group membership system-message text generation from Go backend to the Angular client while preserving the current appearance and actions.

**Architecture:** Add a structured `event` field to `GroupMessage` and its public JSON view. Backend persists/publishes only the event and an empty body for new membership messages. The client maps known events to Russian copy and falls back to `body` for legacy rows.

**Tech Stack:** Go 1.26+, SQLite, existing SSE/publicview layer, Angular 21, TypeScript, Vitest.

## Global Constraints

- No source file may exceed 300 lines.
- No source directory may contain more than 5 source files.
- Preserve the existing system-message presentation and actions.
- Do not change backend API error text in this task.
- Keep old persisted messages readable through a body fallback.

### Task 1: Add structured backend event data

**Files:**
- Modify: `backend/internal/model/model.go` — add optional `Event` to `GroupMessage`.
- Modify: `backend/internal/publicview/publicview.go` — expose `event` in messages and references.
- Modify: `backend/internal/groups/messages.go` — persist membership event identifiers and stop composing Russian text.
- Modify: backend group/message tests — assert event values and empty generated body.

**Interfaces:**
- `model.GroupMessage.Event` is `member_joined`, `member_left`, or `member_removed` for new membership messages.
- `publicview.Message` exposes optional JSON `event`.

- [ ] Write a failing test for `addMembershipMessage` or the existing group integration flow asserting `event=member_joined` and that `body` is empty.
- [ ] Run the focused Go test and verify it fails because the event field is absent and the body still contains generated text.
- [ ] Add the model/public-view field and set `Event: action`, `Body: ""` in the membership-message builder; leave ordinary chat messages unchanged.
- [ ] Update message-reference mapping so replies retain the event metadata without changing displayed legacy bodies.
- [ ] Run `gofmt` and the focused backend tests; verify they pass.

### Task 2: Move copy selection into the web client

**Files:**
- Modify: `web-client/src/app/core/api/models.ts` — add the system event union and optional `event` field.
- Modify: `web-client/src/app/core/events/groups.store.ts` — map known events to the existing visible copy, with legacy body fallback.
- Test: `web-client/src/app/core/events/groups.store.spec.ts` or the established group-message spec — cover all three events and legacy fallback.

**Interfaces:**
- `GroupMessage.event?: GroupSystemEvent`.
- `systemMessageText(item)` returns the existing Russian phrase plus `author.display_name` for known events, otherwise `item.body`.

- [ ] Write failing client tests for joined, left, removed, and an event-less legacy system message.
- [ ] Run the focused Angular test and verify the new assertions fail.
- [ ] Implement the event-to-copy map in the existing message-view boundary; do not change component markup or CSS.
- [ ] Run the focused Angular test and development type/build check; verify they pass.

### Task 3: Contract and regression verification

**Files:**
- Modify documentation only if the public event contract needs a user-facing API note.
- No new source files unless an existing test boundary requires one.

- [ ] Run backend `gofmt -w .`, `go test ./...`, and `go vet ./...`.
- [ ] Run frontend `npm test -- --watch=false` and `npm run build`.
- [ ] Run `git diff --check`.
- [ ] Verify every non-exempt source file is at most 300 lines and every source directory has at most 5 source files.
- [ ] Inspect the final diff for unchanged visual behavior and confirm unrelated error messages were not modified.
