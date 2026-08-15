# Mobile Viewport Lock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Keep the web client fixed to the mobile viewport, including while the on-screen keyboard is open, while preserving scrolling inside intentional app panels.

**Architecture:** Reuse the existing `visualViewport` height service and shell height binding. Add a focused viewport-lock CSS layer to the global app shell so document scrolling and scroll chaining stop at the app boundary; existing message, chat, group, and dialog scroll containers remain the only vertical scroll surfaces.

**Tech Stack:** Angular, TypeScript, CSS, Jasmine/Karma.

## Global Constraints

- Keep source files below 300 lines and directories at or below 5 source files.
- Do not disable scrolling inside `.message-list`, `.group-list`, `.settings-view`, or dialog content.
- Do not add a touch-event interceptor that would break nested scrolling.

---

### Task 1: Lock the document viewport

**Files:**
- Modify: `web-client/src/styles.css`
- Modify: `web-client/src/styles/base.css`
- Test: `web-client/src/app/features/shell/viewport/viewport-height.spec.ts`

**Interfaces:**
- Consumes: existing `getViewportHeight` behavior and `--app-height` CSS variable.
- Produces: a fixed app viewport with `overscroll-behavior: none` at the document/app boundary.

- [ ] **Step 1: Add the failing viewport regression test**

Extend the viewport-height spec with the keyboard case: when the visual viewport is shorter than the layout viewport, `getViewportHeight` returns the visual height and does not allow a negative/zero value.

- [ ] **Step 2: Run the focused test and verify the expected failure**

Run: `npm test -- --include='src/app/features/shell/viewport/viewport-height.spec.ts'`

Expected: the new keyboard edge-case assertion fails before the implementation change.

- [ ] **Step 3: Implement the minimal CSS viewport lock**

Update the global document and app-shell rules so `html`, `body`, `app-root`, and `.messenger` occupy the viewport, hide document overflow, disable scroll chaining, and use `--app-height` for the app height. Keep existing nested `overflow: auto` rules unchanged.

- [ ] **Step 4: Run the focused test and web checks**

Run: `npm test -- --include='src/app/features/shell/viewport/viewport-height.spec.ts'`

Expected: PASS.

- [ ] **Step 5: Commit the implementation**

```bash
git add web-client/src/styles.css web-client/src/styles/base.css web-client/src/app/features/shell/viewport/viewport-height.spec.ts
git commit -m "fix: lock mobile web viewport"
```

### Task 2: Verify responsive behavior and limits

**Files:**
- Modify: none beyond Task 1.

- [ ] **Step 1: Run the complete web test suite**

Run: `npm test`

Expected: all existing and new tests pass.

- [ ] **Step 2: Build the web client**

Run: `npm run build`

Expected: exit code 0; record any pre-existing bundle budget warning.

- [ ] **Step 3: Verify source limits and diff hygiene**

Run: `cd .. && ./scripts/verify-source-limits.sh && git diff --check`

Expected: both checks pass and no source file exceeds 300 lines.
