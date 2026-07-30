# Security Remediation Report
Это короче чат гпт чета исправил я не шарю
Date: 2026-07-30

## Fixed boundaries

1. **Anonymous rate-limit isolation.** Неаутентифицированные запросы больше не
   попадают в общий `session:anonymous` bucket. Проверяются global и IP buckets;
   session bucket добавляется только при наличии токена.
2. **Room-code entropy.** Новые временные комнаты используют 26 символов из
   32-символьного алфавита (130 бит). Migration 9 удаляет старые короткие комнаты.
3. **Session lifecycle.** Добавлена таблица `sessions`, абсолютный и idle TTL,
   обновление активности, cleanup, серверный logout/revoke и `HttpOnly`,
   `SameSite=Strict`, опционально `Secure` cookie. Токен не возвращается в JSON.
4. **Cookie request integrity.** Небезопасные cookie-auth запросы с чужим Origin
   отклоняются; Bearer API clients сохраняют совместимость.
5. **Password policy.** Новая регистрация принимает пароли длиной 10–72 байта; вход сохраняет совместимость со старыми аккаунтами, но ограничивает пароль 72 байтами.
6. **Image decompression DoS.** До полного decode выполняется `DecodeConfig` и
   проверяются формат, максимальные размеры и число пикселей. Результат
   нормализуется в ограниченный JPEG.
7. **Group avatar/storage abuse.** Групповой avatar ограничен коротким символом
   или emoji; legacy values длиннее 8 символов нормализуются миграцией.
8. **Discovery amplification.** Добавлены `LIMIT/OFFSET`, предел 50 результатов,
   offset до 1000 и границы поисковой строки.
9. **Repository hygiene.** Удалены env-файлы, бинарники, dependencies, dist,
   browser logs и шрифты без приложенной лицензии; добавлены `.gitignore`,
   безопасные env examples и MIT license.

## Regression coverage

Добавлены проверки на изоляцию anonymous login limits по IP, отзыв токена после
logout, очистку cookie, idle/absolute expiry, CSRF Origin, oversized declared PNG
dimensions, ограничения group avatar/discovery и 130-битный room code.

## Remaining operational risks

- SQLite rate limiting рассчитан на один backend instance. Для горизонтального
  масштабирования нужен общий store вроде Redis.
- Application limits не заменяют provider/CDN/firewall anti-DDoS.
- Production обязан использовать HTTPS, `SESSION_COOKIE_SECURE=true`, точный
  `CORS_ALLOWED_ORIGINS`, случайные LiveKit credentials и private secret workflow.
- Реальное WebRTC/ICE/TURN поведение проверяется только в полном deployment.


## Verification performed

- `npm run build` — passed (TypeScript check and Vite production build).
- `gofmt -d` and `bash -n start.sh` — passed.
- Compile-only Go check of all changed packages and `cmd/api` using local interface-compatible dependency stubs — passed. The stubs were temporary and are not included in the archive.
- Executable avatar regression tests — passed, including rejection of a PNG declaring 100000×100000 pixels and acceptance of a normal PNG.
- Source limits — maximum 296 lines; no directory exceeds five source files.
- Strong credential-signature scan and repository-hygiene scan — passed.

## Checks blocked by the execution environment

- `go test ./...` and `go vet ./...` could not download the required Go 1.26 toolchain because DNS access to `proxy.golang.org` was blocked.
- `npm audit --omit=dev` reached the configured registry, but its audit endpoint returned HTTP 404.
- `govulncheck` was not installed and could not be downloaded in the restricted environment.

Run those three checks in normal CI before a production deployment. The source-level fixes are present, but the unavailable full backend/runtime and dependency scans must not be hand-waved away.
