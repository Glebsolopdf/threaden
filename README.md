# Threaden

Threaden — вокруг меня крутятся токены

## Быстрый запуск

Требуются Docker Compose, Go 1.26+ для локальных backend-проверок и Node.js 20.19+, 22.12+ или 24+
для Angular web-клиента.

```bash
./start.sh
```

Приложение откроется на `http://127.0.0.1:4200`. Для локальной разработки
используется единый IPv4 loopback-origin: это не даёт Firefox смешивать
IPv6-origin страницы с IPv4 WebRTC/LiveKit endpoint. Подробности находятся в
[`backend/README.md`](backend/README.md), [`web-client/README.md`](web-client/README.md)
и [`deploy/README.md`](deploy/README.md).

Демонстрация экрана (три capture-профиля, диагностика и production-порты)
описана в [`docs/screen-sharing.md`](docs/screen-sharing.md).

Для production Linux/systemd используйте [`threadenctl.sh`](threadenctl.sh):

```bash
sudo ./threadenctl.sh doctor
sudo ./threadenctl.sh start
sudo ./threadenctl.sh status
```

Команды `recovery`, `restart`, `stop|shutdown`, выбор `--backend`, `--web` и
`--livekit` описаны в [`deploy/SYSTEMD.md`](deploy/SYSTEMD.md). Обычный
`restart` только перезапускает сервисы; для полной пересборки используйте
`restart --full`.

## Безопасность

- браузерная сессия хранится в `HttpOnly`, `SameSite=Strict` cookie;
- сессии имеют абсолютный и idle TTL и отзываются сервером при logout;
- пароли принимаются длиной 6–72 символа и хешируются bcrypt;
- временные комнаты используют 130-битные непредсказуемые коды;
- аватары проверяются по MIME и размерам до полного декодирования;
- discover имеет лимиты запроса и пагинацию;
- rate limiting разделён по IP, сессии и пользователю без общего anonymous bucket.

Не коммитьте `.env`, production-секреты, базы, `node_modules`, `dist` или
собранные бинарники. Для сообщения об уязвимости не создавайте публичный issue:
свяжитесь с владельцем репозитория приватно.

## Лицензия

BSD-3 Clause. См. [`LICENSE`](LICENSE).
