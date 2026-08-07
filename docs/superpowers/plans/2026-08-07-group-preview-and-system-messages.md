# Group Preview and System Messages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add public-group message previews, authorized public profiles, persistent actionable membership messages, and private-group history isolation for new members.

**Architecture:** Extend `group_messages` with a backwards-compatible `kind` field. The group service decides visibility and membership cutoffs, while the store performs bounded reads and message persistence. Angular keeps one `MessageView` stream; system messages use the same API actions and only change presentation/copy.

**Tech Stack:** Go 1.26+, SQLite, chi HTTP API, Angular 21 standalone components, signals, Vitest/Karma-compatible Angular test runner, existing CSS tokens.

## Global Constraints

- No source file may exceed 300 lines; ordinary files should remain below 220–250 lines.
- No source directory may contain more than 5 source files.
- Private group data must never be returned to non-members.
- Existing chat messages remain `chat` when the new field is absent.
- Run formatter, tests, type/build checks, vet, and limit verification before completion.

---

### Task 1: Persist message kinds and enforce history visibility

**Files:**
- Modify: `backend/internal/model/model.go` — add `Kind` to `GroupMessage`.
- Create: `backend/internal/store/schema/migration18.go` — add `kind` with default `chat` and index if useful.
- Modify: `backend/internal/store/schema/migrations.go` — register migration 18.
- Modify: `backend/internal/store/groupmessages/messages.go` — read/write `kind` and preserve legacy rows.
- Modify: `backend/internal/store/groups.go` — add member join cutoff lookup and pass cutoff to message query.
- Test: `backend/internal/integration/groups_test.go` — public preview and private history boundary.

**Interfaces:**
- `GroupMessage.Kind` is the JSON/API value `chat` or `system`.
- `Store.Messages(ctx, groupID, cutoff, limit, reader)` continues to return `[]model.GroupMessage`, with `Kind` populated.
- Add `Store.GroupMemberJoinedAt(ctx, groupID, userID) (time.Time, error)`.

- [ ] **Step 1: Write failing integration tests**

Add tests that create a public group, insert a message as a member, assert an unauthenticated `GET /v1/groups/{id}/messages` returns it, then create a private group, insert a message before a member joins and another after joining, and assert the member sees only the latter.

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd backend && go test ./internal/integration -run 'TestGroupsPrivacyInviteAndMessages|TestPrivateGroupHistoryStartsAtJoin' -count=1`

Expected: the public messages request is forbidden and the private history test is absent/fails before implementation.

- [ ] **Step 3: Add migration and store support**

Use `ALTER TABLE group_messages ADD COLUMN kind TEXT NOT NULL DEFAULT 'chat'`, validate scanned values by mapping unknown/empty values to `chat`, and include the field in `groupmessages.Add`/`List`/`Get`. Implement the join timestamp query from `group_members.joined_at`.

- [ ] **Step 4: Make service message reads policy-aware**

Change `Service.Messages` so public groups use the requested preview regardless of membership, private groups require membership, and private members receive a cutoff at `joined_at`. Keep read receipts only for an authenticated member.

- [ ] **Step 5: Run focused tests to verify green**

Run: `cd backend && gofmt -w internal/model/model.go internal/store/schema/migration18.go internal/store/schema/migrations.go internal/store/groupmessages/messages.go internal/store/groups.go internal/groups/messages.go && go test ./internal/integration -run 'TestGroupsPrivacyInviteAndMessages|TestPrivateGroupHistoryStartsAtJoin' -count=1`

- [ ] **Step 6: Commit the persistence/access slice**

`git add backend && git commit -m "feat(groups): add public preview and private history boundaries"`

### Task 2: Create durable membership messages and relax public profiles

**Files:**
- Create: `backend/internal/groups/system_messages.go` — build and persist membership messages.
- Modify: `backend/internal/groups/service.go` — call system-message creation for join/leave/remove and publish the stored message.
- Modify: `backend/internal/groups/profiles.go` — permit authenticated public profiles while retaining private membership checks.
- Modify: `backend/internal/publicview/publicview.go` — expose `kind` and map system messages consistently.
- Modify: `backend/internal/integration/groups_test.go` — persistence, public profile, owner deletion and reply coverage.

**Interfaces:**
- `Service.addMembershipMessage(ctx, groupID, actor, action) (model.GroupMessage, error)` persists a `system` message with a stable human-readable body.
- `publicview.MessageView` includes `kind`.

- [ ] **Step 1: Write failing tests**

After join and leave operations, request history after reopening and assert `kind=system` plus the expected participant text. Assert an authenticated public outsider receives `GET /profile`, while a private outsider still receives 403. Assert the owner can delete the system message and a member can reply to one.

- [ ] **Step 2: Run the focused tests and observe failure**

Run: `cd backend && go test ./internal/integration -run 'TestGroupProfileAndDeletion|TestMembershipMessagesPersist|TestPublicProfileRequiresAuth' -count=1`

- [ ] **Step 3: Implement system-message persistence**

Use the actor user as `Author`, set `Kind: "system"`, generate the body for joined/left/removed actions, call `Store.AddMessage`, and publish `message_created` with `publicview.MessageView`. Keep the existing membership SSE event for live member updates.

- [ ] **Step 4: Apply the profile policy**

In `Profile`, load the group first; return `ErrForbidden` only when the group is private and the caller is not a member. Keep member lists and spam warnings available for the authorized public profile, matching the existing endpoint response.

- [ ] **Step 5: Run focused backend tests**

Run: `cd backend && gofmt -w internal/groups/system_messages.go internal/groups/service.go internal/groups/profiles.go internal/publicview/publicview.go && go test ./internal/integration ./internal/httpapi/groupchat -count=1`

- [ ] **Step 6: Commit the backend behavior**

`git add backend && git commit -m "feat(groups): persist membership messages and public profiles"`

### Task 3: Make message actions and SSE use system messages

**Files:**
- Modify: `backend/internal/httpapi/groupchat/handler.go` — accept system messages through existing reply/delete endpoints without special-casing them away.
- Modify: `backend/internal/groups/messages.go` — validate replies against any message kind and keep owner deletion rule.
- Modify: `web-client/src/app/core/api/models.ts` — add `kind` to `GroupMessage`.
- Modify: `web-client/src/app/core/events/event-stream.service.ts` — merge `message_created` system messages as normal messages and retain membership state updates.
- Modify: `web-client/src/app/core/events/groups.store.ts` — stop synthesizing ephemeral membership messages; expose system messages in common actions.
- Test: `backend/internal/httpapi/groupchat/handler_test.go` and `web-client/src/app/core/events/groups.store.spec.ts`.

- [ ] **Step 1: Write failing tests**

Cover that a system message returned by the API can be used as `reply_to_id`, owner deletion succeeds, and an SSE `message_created` event survives in the client message list without creating a duplicate membership-only item.

- [ ] **Step 2: Run tests to verify red**

Run: `cd backend && go test ./internal/httpapi/groupchat -run 'TestSystemMessage' -count=1`; then `cd web-client && npm test -- --runInBand` for the focused spec if supported by the project runner.

- [ ] **Step 3: Implement the common message path**

Ensure the client action menu receives system messages, copy uses `message.body`, reply uses `message.id`, and deletion authorizes owner/group owner exactly as backend does. Membership SSE remains responsible only for member lists/notifications, while persisted message SSE owns chat history.

- [ ] **Step 4: Run backend and frontend focused tests**

Run the two commands above and correct type errors without weakening assertions.

- [ ] **Step 5: Commit the message action slice**

`git add backend web-client && git commit -m "feat(chat): make system messages actionable"`

### Task 4: Add preview/history banners and loading animation

**Files:**
- Modify: `web-client/src/app/core/events/groups.store.ts` — track whether current view is preview and whether private history was cut at join.
- Modify: `web-client/src/app/features/groups/group.component.ts` — render explanatory banners, disable composer for preview, and open public profile for authorized outsiders.
- Modify: `web-client/src/app/features/groups/group-message-list.component.ts` — render system kind with shared actions and resilient text.
- Modify: `web-client/src/styles/screens/groups/shell.css` — replace static loading copy with staged skeleton animation.
- Modify: `web-client/src/styles/screens/groups/chat.css` — style preview/history banners and actionable system messages.
- Test: `web-client/src/app/features/groups/group-message-list.component.spec.ts` — system action rendering and loading/preview states.

**Visual direction:** Use Threaden’s existing dark surface and accent variables. The banner is a compact “window into the room”: a translucent accent rail, a small live-dot/lock glyph, one sentence of plain copy, and a single join action. The loading state reveals the avatar ring, title line, metadata line, and three staggered message bars with a 900ms shimmer; respect the existing reduced-motion stylesheet. This keeps the chat identity visible during loading without introducing a new visual system.

- [ ] **Step 1: Write failing component tests**

Assert that a public non-member sees preview copy and a join CTA, a private new member sees the hidden-history copy, and a system message renders action controls instead of being an inert plain span.

- [ ] **Step 2: Run focused frontend tests to verify red**

Run: `cd web-client && npm test -- --watch=false`

Expected: the new assertions fail before template/store/style changes.

- [ ] **Step 3: Implement store flags and templates**

Derive preview from `current.visibility === 'public' && !currentIsMember()`. Derive history notice from private membership plus the earliest message being at/after the server’s join cutoff; expose a small API field if the existing response cannot determine it without guessing. Render banners above the list and use the same group action component for system messages.

- [ ] **Step 4: Implement intentional loading motion and system styling**

Add semantic classes, staggered animation delays, focus-visible states, and reduced-motion overrides using existing tokens and animation conventions.

- [ ] **Step 5: Run frontend tests and development build**

Run: `cd web-client && npm test -- --watch=false && npm run lint:types`

- [ ] **Step 6: Commit the UI slice**

`git add web-client && git commit -m "feat(web): explain group previews and history boundaries"`

### Task 5: Full verification, restart, and delivery

**Files:**
- Modify: `README.md` or `backend/README.md` only if the final API behavior requires documented public contract changes.
- No new verification tool unless an existing project command is unavailable.

- [ ] **Step 1: Run backend checks**

`cd backend && gofmt -w . && go test ./... && go vet ./...`

- [ ] **Step 2: Run frontend checks**

`cd web-client && npm test -- --watch=false && npm run build`

- [ ] **Step 3: Verify architectural limits**

Use a temporary read-only shell inspection to report the largest non-exempt source file and every source directory with five files; fail if any file exceeds 300 lines or directory exceeds five source files.

- [ ] **Step 4: Inspect diff and run the requested service restart**

Run `git diff --check`, `git status --short`, then `sudo ./threadenctl.sh restart --backend --web` (or the repository’s supported equivalent if sudo is unavailable). Query `/healthz` and the web endpoint after restart.

- [ ] **Step 5: Commit any final documentation/verification adjustments**

Create one final commit only for scoped adjustments, then push the feature commits to the configured GitHub remote with `git push origin main`.

- [ ] **Step 6: Report exact evidence**

Include changed files, commit hashes, commands and exit results, restart status, limit maximums, untouched `design-previews/` user files, and any unverified behavior.
