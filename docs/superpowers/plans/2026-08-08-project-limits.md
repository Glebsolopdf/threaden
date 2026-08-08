# Project Limits Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove all current source-file and source-directory limit violations while preserving behavior and verifying the result with the project's backend, frontend, deployment, and structural checks.

**Architecture:** Extract by responsibility, not by line ranges. Go packages will use domain subpackages for cleanup, groups, and users while keeping explicit composition at package boundaries; Angular service and CSS files will be split along existing state, transport, and visual concepts.

**Tech Stack:** Go 1.22+ toolchain, Angular/TypeScript, CSS, Bash, systemd/nginx, SQLite-backed backend.

## Global Constraints

- Source files must remain below the hard ceiling of 300 lines.
- Source directories must contain no more than 5 source files.
- Preserve public behavior and existing API signatures unless an import path must change because of a real package boundary.
- Do not modify `design-previews/` or unrelated working-tree content.
- Use `apply_patch` for source edits and run tests after each cohesive extraction.

---

### Task 1: Establish the toolchain and baseline

**Files:**
- Modify: none
- Test: repository commands only

**Interfaces:**
- Consumes: current `main` checkout and existing dependencies.
- Produces: installed Go toolchain and captured baseline test/limit results.

- [ ] **Step 1: Install Go if absent**

Run `sudo apt-get update && sudo apt-get install -y golang-go` when `go` is not available. Verify with `go version`.

- [ ] **Step 2: Run baseline checks**

Run `cd backend && go test ./...`, `go vet ./...`, `cd ../web-client && npm test -- --no-progress`, and `npm run build`. Record any pre-existing failures before changing code.

- [ ] **Step 3: Confirm the working tree boundary**

Run `git status --short` and preserve the untracked `design-previews/` directory.

### Task 2: Split the Angular voice service

**Files:**
- Modify: `web-client/src/app/core/voice/voice.service.ts`
- Create: `web-client/src/app/core/voice/voice-state.ts`
- Create: `web-client/src/app/core/voice/voice-events.ts`
- Create: `web-client/src/app/core/voice/voice-media.ts`
- Modify: `web-client/src/app/core/voice/activity-detector.spec.ts` only if the extracted voice-state boundary needs a focused regression assertion.

**Interfaces:**
- Consumes: existing LiveKit client, event stream, auth store, and service callers.
- Produces: the existing `VoiceService` public methods and state; focused private helpers move to modules with typed functions.

- [ ] **Step 1: Add a failing boundary test**

Add a focused test that exercises the existing public service behavior through the extracted state/event boundary, then run the targeted Angular test and confirm the new boundary is not yet available.

- [ ] **Step 2: Extract state and roster transformations**

Move signal/map state and pure roster/member transformations into `voice-state.ts`; keep the service as the coordinator.

- [ ] **Step 3: Extract event handling**

Move event subscription and event-to-state updates into `voice-events.ts` with explicit inputs and callbacks.

- [ ] **Step 4: Extract media/room operations**

Move LiveKit room and media lifecycle helpers into `voice-media.ts` without changing service method names or call sites.

- [ ] **Step 5: Run frontend tests**

Run `npm test -- --no-progress` and `npm run build`; fix only extraction regressions.

### Task 3: Split oversized stylesheets

**Files:**
- Modify: `web-client/src/styles/usability.css`
- Modify: `web-client/src/styles/customization/dialogs.css`
- Modify: `web-client/src/styles/screens/groups/voice-room.css`
- Create: concept-specific stylesheet files beside each original file.
- Modify: `web-client/src/styles/main.css` for the split stylesheet imports; `web-client/src/styles.css` remains unchanged unless a selector currently imported there must move with its owning concept.

**Interfaces:**
- Consumes: existing selector names and import order.
- Produces: identical CSS cascade and visual behavior, with each stylesheet below 300 lines.

- [ ] **Step 1: Add/adjust a frontend smoke assertion**

Use the existing Angular build and component tests as the red/green safety net; do not introduce snapshot-only assertions for CSS.

- [ ] **Step 2: Extract each stylesheet by concept**

Separate accessibility/usability primitives, dialog groups, and voice-room subfeatures into named files such as `usability/forms.css`, `customization/dialogs/overlays.css`, and `groups/voice-room/participants.css`; preserve selector text and import order.

- [ ] **Step 3: Verify CSS extraction**

Run `npm test -- --no-progress` and `npm run build`, then re-run the structural checker.

### Task 4: Split the groups package directory

**Files:**
- Modify: `backend/internal/groups/service.go`
- Move or create: `backend/internal/groups/cleanup/cleanup.go`
- Modify: callers that construct cleanup configuration if the package path changes.
- Test: existing `backend/internal/integration/*groups*` and cleanup tests.

**Interfaces:**
- Consumes: `store.Store`, group service clock/configuration, and existing cleanup callers.
- Produces: the same cleanup behavior through an explicit cleanup package boundary; root `groups` stays at five or fewer source files.

- [ ] **Step 1: Add a failing package-boundary test**

Add a test for default cleanup configuration and emergency cleanup behavior at the boundary that will remain public.

- [ ] **Step 2: Extract cleanup types and operations**

Move cleanup configuration, stats, and cleanup operations into `backend/internal/groups/cleanup/`; expose only the functions/types required by the root service composition.

- [ ] **Step 3: Reconnect the service**

Update `Service` construction and cleanup invocation with explicit imports; avoid a circular dependency by passing the store and clock/configuration into the cleanup operation.

- [ ] **Step 4: Run backend tests**

Run `cd backend && go test ./...` and `go test -race ./...`.

### Task 5: Split the store package directory

**Files:**
- Modify: `backend/internal/store/store.go`
- Move/create: `backend/internal/store/users/users.go`
- Move/create: `backend/internal/store/groups/groups.go`
- Move/create: `backend/internal/store/groups/profiles.go`
- Move/create: `backend/internal/store/groups/security.go`
- Modify: callers and package composition files required by the new boundaries.

**Interfaces:**
- Consumes: existing SQLite handle, models, and store error contracts.
- Produces: explicit user/group persistence subpackages with the same application-facing operations and error semantics.

- [ ] **Step 1: Add focused persistence contract tests**

Extend existing store tests to cover the operations whose package ownership changes: profile update, group profile loading, and group security transitions.

- [ ] **Step 2: Extract user persistence**

Move user/session operations into `store/users/` and inject the database dependency through a small concrete store type; preserve error mapping at the root boundary.

- [ ] **Step 3: Extract group persistence**

Move group, group-profile, and group-security operations into `store/groups/`; keep room persistence in the root or a separate domain directory so no directory exceeds five files.

- [ ] **Step 4: Reconnect application and tests**

Update imports and constructors, then run `go test ./...`, `go test -race ./...`, and `go vet ./...`.

### Task 6: Add structural verification and document commands

**Files:**
- Create: `scripts/verify-source-limits.sh`
- Modify: `README.md` or relevant developer documentation with the exact command.

**Interfaces:**
- Consumes: repository source tree.
- Produces: deterministic exit-1 verification when a non-exempt source file exceeds 300 lines or a source directory exceeds five files.

- [ ] **Step 1: Write failing verifier cases**

Run the verifier against the current tree and confirm it reports the known violations.

- [ ] **Step 2: Implement the verifier**

Use local `find`/`awk`/shell logic, exclude `.git`, dependency, generated, cache, and lockfile paths, and print every violation.

- [ ] **Step 3: Run the verifier on the refactored tree**

Run `./scripts/verify-source-limits.sh`; expected result is exit 0 with no violations.

### Task 7: Verify, restart public web, commit, and push

**Files:**
- Modify: none beyond prior tasks.

- [ ] **Step 1: Run the full verification set**

Run `cd backend && go test ./... && go test -race ./... && go vet ./...`; run `cd ../web-client && npm test -- --no-progress && npm run build`; run `./scripts/verify-source-limits.sh`; run `git diff --check`.

- [ ] **Step 2: Rebuild and restart the public service**

Run `sudo ./threadenctl.sh restart --full --public`, then verify `systemctl is-active threaden-public-web.service`, `curl --fail http://127.0.0.1:18082/`, and nginx config validation through the deployment script.

- [ ] **Step 3: Inspect the final diff**

Run `git status --short`, `git diff --stat`, and `git diff --check`; confirm `design-previews/` is not staged.

- [ ] **Step 4: Commit and push**

Run `git add` only for the refactor, tests, verifier, and documentation; commit with `refactor: enforce source file and directory limits`; push with `git push origin main`.
