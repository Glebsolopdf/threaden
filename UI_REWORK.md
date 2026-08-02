# Threaden 0.3.2 — UI repair

This release repairs the Angular client layout while preserving the existing backend, API routes, SSE flow, LiveKit integration, and application state stores.

## Restored from the 0.2.7 client

- Three-column desktop messenger shell: navigation rail, group list, workspace.
- Group list/profile chrome and connection indicator.
- Chat bubbles, fixed composer, group voice strip, voice-room dialogs, device menu.
- Mobile off-canvas navigation and stable viewport-height handling.
- Group profile/member dialog styling.

## Angular-specific fixes

- Angular route component hosts are block-level and fill the router workspace.
- Route and sidebar state are applied to `.messenger`, matching the CSS selectors.
- Shell page mode follows `NavigationEnd` and closes mobile navigation on route changes.
- Visual viewport height is maintained without collapsing the app when the software keyboard opens.
- Missing legacy theme files are imported explicitly rather than relying on side-effect TypeScript imports.

## Usability changes

- Separate group-list heading and search area.
- Active group marker and clearer empty/search states.
- Restored profile footer with textual connection state.
- New home workspace with quick actions.
- Card-based settings layout and repaired discover-page scrolling.
- Bottom-sheet dialogs on narrow screens.
- Improved focus rings, scrollbars, touch targets, and responsive behavior.

## Manual smoke test

1. Run `./start.sh` and sign in.
2. Check `/`, `/discover`, `/settings`, and `/profile` at desktop and mobile widths.
3. Open a group, send messages, open group details, and open the voice-room list.
4. At a width below 896 px, open and close the navigation drawer; press Escape to close it.
5. Join a voice room and verify that the navigation panels disappear in full-screen voice mode.
