# Threaden

Threaden — вокруг меня крутятся токены

## Быстрый запуск

Требуются Docker Compose, Go 1.26+ для локальных backend-проверок и Node.js 24+
для web-клиента.

```bash
./start.sh
```

Приложение откроется на `http://localhost:5173`. Подробности находятся в
[`backend/README.md`](backend/README.md), [`web-client/README.md`](web-client/README.md)
и [`deploy/README.md`](deploy/README.md).

Для production Linux/systemd используйте [`threadenctl.sh`](threadenctl.sh):

```bash
sudo ./threadenctl.sh doctor
sudo ./threadenctl.sh start
sudo ./threadenctl.sh status
```

Команды `recovery`, `restart`, `stop|shutdown`, выбор `--backend`, `--web` и
`--livekit` описаны в [`deploy/SYSTEMD.md`](deploy/SYSTEMD.md).

## Безопасность 

- браузерная сессия хранится в `HttpOnly`, `SameSite=Strict` cookie;
- сессии имеют абсолютный и idle TTL и отзываются сервером при logout;
- пароли принимаются длиной 10–72 байта и хешируются bcrypt;
- временные комнаты используют 130-битные непредсказуемые коды;
- аватары проверяются по MIME и размерам до полного декодирования;
- discover имеет лимиты запроса и пагинацию;
- rate limiting разделён по IP, сессии и пользователю без общего anonymous bucket.

Не коммитьте `.env`, production-секреты, базы, `node_modules`, `dist` или
собранные бинарники. Для сообщения об уязвимости не создавайте публичный issue:
свяжитесь с владельцем репозитория приватно.

## Лицензия

MIT. См. [`LICENSE`](LICENSE).
