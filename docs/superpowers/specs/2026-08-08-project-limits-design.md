# Project Source Limits Design

## Goal

Bring every existing non-generated source file below 300 lines and every source directory to at most five files without changing product behavior.

## Scope

The change covers the four oversized source files and the two directories currently above the architectural limits. Generated assets, dependency directories, lockfiles, and the pre-existing untracked `design-previews/` directory remain out of scope.

## Design

- Split `voice.service.ts` by lifecycle responsibility while preserving the service's public methods and observable state used by components and tests.
- Split the oversized stylesheets by their existing visual concepts, keeping the same selectors and import order.
- Move group cleanup into a `groups/cleanup/` package and store-related domain files into meaningful `store/groups/` and `store/users/` packages. Keep the root packages as composition boundaries where callers already depend on them.
- Add focused regression tests only where extraction changes a boundary; existing behavior tests remain authoritative.
- Add a repeatable limit verification command to the implementation checks, but do not add a new runtime dependency for it.

## Non-goals

- No feature behavior changes.
- No dependency upgrades unrelated to the refactor.
- No deletion or modification of untracked design assets.
- No deployment configuration changes beyond rebuilding and restarting the public web service.

## Acceptance Criteria

- Every non-exempt source file is at most 300 lines.
- Every non-exempt source directory contains at most five source files.
- Backend tests, race tests, vet, and Angular tests/build/type checks pass.
- `threaden-public-web.service` is rebuilt/restarted and responds successfully on its configured loopback port.
- The final GitHub commit contains only this refactor and its verification/documentation changes.
