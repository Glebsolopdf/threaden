# Threaden Angular web client

Версия: 0.3.3

Веб-клиент переписан с Vanilla TypeScript/Vite на Angular. Серверные контракты не изменены: приложение продолжает использовать Go API, HttpOnly cookie, SSE `/v1/events` и LiveKit.

## Стек

- Angular 21, standalone components и lazy routes;
- Signals/computed/effect для состояния интерфейса;
- RxJS для HTTP, SSE-интеграции и debounce поиска;
- Reactive Forms для входа, регистрации, профиля, групп и комнат;
- функциональный HttpClient interceptor с `credentials: include`;
- zoneless change detection и `OnPush`-компоненты;
- LiveKit для аудиокомнат;
- runtime-конфигурация API через `public/runtime-config.js`.

Angular 21 выбран намеренно: он совместим с Node `22.16`, который использовался при миграции. Angular 22 требует Node `22.22.3+`.

## Запуск

```bash
npm install
npm run dev
```

Angular dev server доступен на `http://127.0.0.1:4200` и проксирует `/v1`,
`/healthz`, `/readyz` на `http://127.0.0.1:8080`. Используйте именно этот URL:
Firefox 153 включает защиту Local Network Access и при открытии dev-сервера
через IPv6 `localhost` (`::1`) может не запустить ICE gathering для IPv4
LiveKit endpoint.

## Production

```bash
npm run build
sudo rsync -a --delete dist/ /var/www/threaden/
```

Nginx должен возвращать `index.html` для неизвестных SPA-маршрутов. Существующий `deploy/nginx.conf` уже делает это через `try_files`.

## Runtime API URL

Для same-origin production оставьте `apiBaseUrl` пустым:

```js
window.__THREADEN_CONFIG__ = { apiBaseUrl: "" };
```

Для отдельного API-origin измените `public/runtime-config.js`. Backend должен разрешать точный origin и credentialed cookies.

## Маршруты

- `/login`, `/register`, `/profile`;
- `/groups/:id`, `/invite/:token`, `/discover`;
- `/temporary`, `/temporary/:code`;
- `/group-voice-rooms/:id`;
- `/settings`.

## Архитектура

- `src/app/core` — API, auth, stores, SSE, preferences и voice engine;
- `src/app/features` — лениво загружаемые страницы;
- `src/app/shared` — avatar, notifications и persistent room widget;
- `src/styles` — сохранённая визуальная система исходного клиента.

В группах можно отправлять до трёх файлов с подписью или без неё. Сервер
проверяет формат по содержимому, обрабатывает фото и видео, принимает архивы до
5 МБ и автоматически удаляет вложения через 72 часа. Клиентские проверки размера
предварительные; окончательное решение всегда принимает backend. В окне аккаунта
раздел «Квоты» показывает ограничения и позволяет запланировать удаление сообщений
с вложениями через 5 минут с возможностью отмены. Для аккаунтов младше 24 часов
раздел показывает временную квоту 20 МБ и суточный лимит 20 МБ; после первых суток
суточный лимит снимается.
