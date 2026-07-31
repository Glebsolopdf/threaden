# Миграция web-клиента Threaden на Angular

## Граница миграции

Backend, API-контракты, cookie-сессия, SSE endpoint `/v1/events`, LiveKit и production nginx не менялись. Переписан только браузерный клиент.

## Архитектура

- **Standalone Angular application** без `NgModule`.
- **Angular Router** с функциональными guards и ленивой загрузкой feature-компонентов.
- **Signals + computed** как основа локального и разделяемого состояния.
- **RxJS** для HTTP, debounce поиска, SSE и асинхронных потоков.
- **Reactive Forms** для входа, регистрации, профиля, настроек, групп, сообщений и голосовых комнат.
- **HttpClient + functional interceptor** для cookies, единообразной обработки ошибок и API base URL.
- **OnPush** во всех компонентах. Angular 21 запускает приложение без `zone.js`.
- **LiveKit service** изолирует WebRTC, устройства, громкость, микрофон и roster от UI.

## Состояние

| Store/service | Ответственность |
|---|---|
| `AuthStore` | текущий пользователь, bootstrap сессии, login/logout |
| `GroupsStore` | группы, выбранная группа, сообщения, участники, приглашения |
| `EventStreamService` | SSE reconnect и типизированные события |
| `VoiceService` | LiveKit Room, участники, устройства, микрофон, persistent room widget |
| `PreferencesService` | настройки аудио/web в localStorage |
| `NotificationStore` | единый поток toast-уведомлений |

## Перенесённые сценарии

- вход, регистрация и выход;
- список групп, создание, вступление, выход и удаление;
- приглашения и discover;
- чат, оптимистическая отправка и повтор ошибки;
- профиль и загрузка аватара с прогрессом;
- настройки устройств, громкости и микрофона;
- временные и групповые голосовые комнаты;
- SSE обновления;
- сворачиваемый и перетаскиваемый виджет активной комнаты.

## Запуск

```bash
npm install
npm run dev
```

Dev-сервер: `http://127.0.0.1:4200`. Запросы `/v1` проксируются на `http://127.0.0.1:8080`.

Production:

```bash
npm run build
```

Результат остаётся в `web-client/dist`, поэтому существующий deployment pipeline не требует смены каталога.

## LiveKit 2.21 declaration compatibility

The LiveKit 2.21.0 package publishes declaration files that reference development-only type packages (`@types/sdp-transform` and `@livekit/throws-transformer`). The application keeps strict checking for its own source code and enables TypeScript `skipLibCheck` only for third-party declaration files. This avoids coupling the application to LiveKit's internal build-time transformer package.
