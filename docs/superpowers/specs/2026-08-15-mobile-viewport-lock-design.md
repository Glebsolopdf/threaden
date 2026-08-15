# Mobile Viewport Lock

## Goal

On phones, the web client must remain fixed to the viewport when the software keyboard opens and must not allow the document itself to move vertically. Intentional inner panels, such as the message list and dialog content, keep their own scrolling.

## Design

The existing `visualViewport` integration remains the source of truth for the app height. Global document and app-shell styles will use that height, `position: fixed`/viewport sizing, `overflow: hidden`, and `overscroll-behavior: none` to prevent browser-level scroll and scroll chaining. Nested scroll containers are left unchanged, so touch scrolling remains available where content requires it.

## Error and compatibility behavior

When `visualViewport` is unavailable, the existing `window.innerHeight` fallback remains active. The change does not intercept touch events and therefore does not interfere with nested scrolling or accessibility input.

## Verification

The viewport-height regression test covers the keyboard-shortened visual viewport. The complete web test suite, production build, source-limit check, and diff check must pass.
