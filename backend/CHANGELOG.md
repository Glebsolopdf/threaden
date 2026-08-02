# Changelog

## 0.3.3 - 2026-08-01

- Replaced the fixed 24-hour IP ban with an escalating ladder
  (10s → 1m → 5m → 24h) that resets after 24h without violations, and added
  automatic account deletion after 5 maximum-level (24h) bans within 30 days.
- Added an anti-flood ownership limit (default 3 groups per account), hid
  groups with fewer than 5 members from discovery, and added automatic 24-hour
  IP bans after repeated rate-limit violations.
- Fixed local Firefox WebRTC connections by using one IPv4 loopback origin for
  Angular and LiveKit, advertising reachable host ICE candidates, and enabling
  LiveKit's embedded TURN/UDP fallback.

## 0.2.5 - 2026-07-30

- Replaced browser Bearer storage with expiring, revocable HttpOnly cookie sessions and CSRF origin checks.
- Replaced enumerable four-character temporary room codes with 130-bit codes and invalidated legacy rooms.
- Fixed the shared anonymous rate-limit bucket denial of service.
- Added safe image dimension checks, bounded group avatars, and paginated discovery.
- Added security regression tests and repository publication hygiene.

- Added low-disk emergency cleanup that triggers below 5 GiB free space and prioritizes inactive accounts, inactive groups, and oldest messages.
- Fixed real-time message updates by keeping server-sent event streams flushable and open for long-lived clients.
- Made `threaden.substituteme.space` the primary production web domain and redirected `opentalk.substituteme.space` to it.

## 0.2.3 - 2026-07-27

- Added profile deletion and avatar reset API endpoints.

## 0.2.2 - 2026-07-27

- Added inactive account cleanup after 7 days since the last authenticated activity.
- Raised avatar upload input size to 8 MiB while keeping server-side JPEG compression.

## 0.2.1 - 2026-07-27

- Added a standalone profile setup page after registration.
- Converted uploaded avatars server-side to compressed square JPEG data URLs.

## 0.2.0 - 2026-07-27

- Added email/password registration, login, persistent Bearer sessions, and profile editing.
- Added avatar upload validation and server-side JPEG conversion/compression for JPEG, PNG, GIF, and WebP images up to 8 MiB.
- Replaced temporary users with registered users and migrated rooms/members to the new table.
- Removed user expiration configuration and temporary-account cleanup.

## 0.1.2 - 2026-07-27

- Fixed local WebRTC connection by advertising LiveKit's reachable host address instead of Docker loopback.

## 0.1.1 - 2026-07-26

- Added a root `start.sh` script that starts the backend, LiveKit, and web client together.
- Added the web-client favicon to avoid the development-server 404.
- Run local LiveKit on the host network so its WebRTC UDP traffic is not blocked by Docker NAT.

## 0.1.0 - 2026-07-26

- Added temporary users authenticated by hashed session tokens.
- Added expiring voice rooms with transactional membership limits.
- Added scoped LiveKit audio JWT generation and participant/room termination.
- Added SQLite migrations, cleanup, HTTP safety middleware, containers, and tests.
