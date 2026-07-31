# Security policy

## Supported version

Security fixes are applied to the latest release in this repository.

## Deployment requirements

- Keep `deploy/production.env` and all `.env` files outside Git.
- Set `SESSION_COOKIE_SECURE=true` behind HTTPS.
- Configure an exact `CORS_ALLOWED_ORIGINS` list; do not use `*` in production.
- Replace the example LiveKit key and secret before starting the stack.
- Run `go test -race ./...`, `go vet ./...`, `govulncheck ./...`,
  `npm install`, `npm run build`, and `npm audit --omit=dev` in CI.
