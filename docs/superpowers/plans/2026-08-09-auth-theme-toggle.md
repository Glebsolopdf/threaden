# Auth Theme Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move dark theme styles into `themes/dark.css` and add a persistent dark/light toggle to login and registration screens.

**Architecture:** Keep `PreferencesService` as the single theme state and persistence boundary. Move only dark theme CSS from foundation variables into a dedicated theme file, and use one standalone auth toggle component in both auth pages so presentation stays shared and auth components remain focused.

**Tech Stack:** Angular 21 standalone components, Signals, localStorage, CSS imports, Vitest.

## Global Constraints

- No source file may exceed 300 lines; ordinary files should remain below 220–250 lines.
- No non-exempt directory may contain more than 5 source files.
- The toggle changes only `dark` and `light`; forest, burgundy, and purple remain available after login.
- Theme selection must persist through the existing `PreferencesService` localStorage key.

---

### Task 1: Lock down theme persistence and toggle behavior

**Files:**
- Create: `web-client/src/app/core/preferences/preferences.service.spec.ts` — test default theme, persisted theme, and `setTheme` DOM/storage behavior.
- Create: `web-client/src/app/features/auth/auth-theme-toggle.component.spec.ts` — test dark/light toggle output through the real `PreferencesService` boundary where practical.

**Interfaces:**
- Consume `PreferencesService.theme()` and `PreferencesService.setTheme(theme)`.
- The component will expose no public API beyond its standalone selector; its click action toggles `dark` to `light` and every non-light theme to `dark`.

- [ ] **Step 1: Write failing service tests**

  Assert an empty storage resolves to `dark`, a stored `light` resolves to `light`, and `setTheme('light')` updates `document.documentElement.dataset.theme` and the `voice_rooms_theme` storage value.

- [ ] **Step 2: Run tests and verify the failure**

  Run `npm test` in `web-client`; confirm failures are limited to the new missing test setup/behavior.

- [ ] **Step 3: Add toggle component tests**

  Assert the component renders an accessible button and toggles the injected preference from dark to light and back. Use the existing Angular TestBed/Vitest setup.

### Task 2: Extract dark CSS and add the shared auth control

**Files:**
- Create: `web-client/src/styles/themes/dark.css` — dark `:root` theme variables currently in `foundation/variables.css`.
- Create: `web-client/src/app/features/auth/auth-theme-toggle.component.ts` — standalone accessible theme toggle using `PreferencesService`.
- Modify: `web-client/src/styles/foundation/variables.css` — retain font-face declarations and remove moved theme variables.
- Modify: `web-client/src/styles/main.css` — import `themes/dark.css` before other theme files.
- Modify: `web-client/src/styles/screens/auth/index.css` — style the toggle and its focus/hover states.
- Modify: `web-client/src/app/features/auth/login.component.ts` — import and render the toggle.
- Modify: `web-client/src/app/features/auth/register.component.ts` — import and render the toggle.

**Interfaces:**
- `AuthThemeToggleComponent` uses `preferences.theme()` and calls `preferences.setTheme(nextTheme)`; it does not duplicate persistence or manipulate CSS directly.

- [ ] **Step 1: Implement the service-backed toggle**

  Render a button with an accessible label such as `Переключить на светлую тему` or `Переключить на тёмную тему`, show the current mode text, and call `setTheme` on click.

- [ ] **Step 2: Move dark variables without duplication**

  Copy the complete `:root` variable block from `foundation/variables.css` to `themes/dark.css`, leave `@font-face` declarations in foundation, and add `@import "./themes/dark.css";` before light/forest/burgundy/purple imports.

- [ ] **Step 3: Place the control on auth screens**

  Add the standalone component to both `imports` arrays and render it near the brand/header without changing form behavior or navigation.

- [ ] **Step 4: Style the control**

  Use existing auth tokens for a compact secondary button, with visible keyboard focus and suitable light/dark contrast. Do not hard-code a second theme system.

### Task 3: Verify integration and structure

**Files:**
- Modify only files from Tasks 1–2 if verification exposes an integration issue.

- [ ] **Step 1: Run all client checks**

  Run `npm test`, `npm run build`, and `git diff --check` in the repository.

- [ ] **Step 2: Inspect theme ownership**

  Confirm dark variables exist in `themes/dark.css`, are not duplicated in `foundation/variables.css`, and auth components include the toggle.

- [ ] **Step 3: Verify source limits**

  Excluding `.git`, `node_modules`, and build output, confirm no source file exceeds 300 lines and no directory contains more than 5 source files.
