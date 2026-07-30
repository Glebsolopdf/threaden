# Threaden web client

Vanilla TypeScript/Vite-клиент. Браузер обращается к Go API, а аудио передаёт
напрямую через LiveKit.

## Запуск

```bash
cp .env.example .env
npm ci
npm run dev
```

Vite проксирует `/v1`, `/healthz` и `/readyz` на `http://127.0.0.1:8080`.
Production-сборка:

```bash
npm run build
npm run preview
```

## Авторизация

Клиент отправляет `fetch(..., { credentials: "include" })`. Сессионный токен
находится в `HttpOnly` cookie и не читается и не сохраняется JavaScript-кодом.
В `localStorage` остаётся только код активной голосовой комнаты для попытки
переподключения после reload. LiveKit JWT не сохраняется.

Production рекомендуется обслуживать с того же origin, который проксирует `/v1`
в backend. При прямом cross-origin API укажите точный origin в
`CORS_ALLOWED_ORIGINS`; wildcard с credentialed cookie не работает.

## Маршруты

- `/login/`, `/register/`, `/profile/` — аккаунт;
- `/groups/{id}` и `/invite/{token}` — группы;
- `/temporary/{code}` — временная комната;
- `/settings/` — настройки внутри SPA.

Временный код содержит 26 символов. Production-сервер должен возвращать
`index.html` для неизвестных SPA-путей.

## Проверки

```bash
npm ci
npm run build
npm audit
```
