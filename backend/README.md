# Threaden backend

Go API хранит пользователей, группы, сообщения и временные комнаты в SQLite и
выдаёт ограниченные LiveKit JWT. Медиа не проходит через Go-процесс.

## Запуск

Из корня проекта:

```bash
./start.sh
```

Только backend и LiveKit:

```bash
cp backend/.env.example backend/.env
cd backend
docker compose up --build
```

Для запуска без Docker нужен Go 1.26+, записываемый SQLite-файл и доступный
LiveKit:

```bash
cd backend
set -a; . ./.env.example; set +a
go run ./cmd/api
```

## Сессии и авторизация

Регистрация и login устанавливают host-only cookie `threaden_session` с
`HttpOnly`, `SameSite=Strict` и опциональным `Secure`. Токен не возвращается в
JSON и не доступен JavaScript. Для CLI остаётся совместимость с заголовком
`Authorization: Bearer <token>`, но браузерный клиент использует cookie.

Сессии имеют:

- абсолютный TTL `SESSION_TTL`;
- idle TTL `SESSION_IDLE_TTL`;
- серверный отзыв через `DELETE /v1/auth/logout`;
- хранение только SHA-256 хеша токена в SQLite.

Пароль должен занимать от 10 до 72 байт. Пароли хранятся как bcrypt-хеши.
Небезопасные cookie-запросы с посторонним `Origin` отклоняются как CSRF.

## Основные переменные

| Переменная | Назначение |
|---|---|
| `HTTP_ADDR` | Адрес HTTP-сервера |
| `DATABASE_PATH` | Путь к SQLite |
| `LIVEKIT_URL` | Внутренний адрес LiveKit |
| `LIVEKIT_PUBLIC_URL` | URL LiveKit для браузера |
| `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET` | Ключ и секрет LiveKit |
| `ROOM_TTL` | Срок жизни временной комнаты |
| `LIVEKIT_TOKEN_TTL` | Максимальный TTL LiveKit JWT |
| `SESSION_TTL` | Абсолютный срок сессии |
| `SESSION_IDLE_TTL` | Срок бездействия сессии |
| `SESSION_COOKIE_SECURE` | Требовать HTTPS для cookie; в production должно быть `true` |
| `MAX_ROOM_PARTICIPANTS` | Лимит участников комнаты |
| `CORS_ALLOWED_ORIGINS` | Разрешённые origins через запятую; в production не используйте `*` |
| `TRUSTED_PROXIES` | Прокси, которым разрешены forwarding-заголовки |
| `RATE_LIMIT_BUCKET_TTL` | TTL persistent rate-limit bucket |

Остальные cleanup- и anti-spam-переменные перечислены в `.env.example`.

## HTTP API

| Метод | Endpoint | Авторизация |
|---|---|---|
| `POST` | `/v1/auth/register` | нет |
| `POST` | `/v1/auth/login` | нет |
| `DELETE` | `/v1/auth/logout` | cookie или Bearer; идемпотентно |
| `GET` | `/v1/me` | сессия |
| `PATCH` | `/v1/me` | сессия, multipart avatar |
| `DELETE` | `/v1/me/avatar` | сессия |
| `DELETE` | `/v1/me` | сессия |
| `POST` | `/v1/rooms` | сессия |
| `GET` | `/v1/rooms/{code}` | сессия |
| `POST` | `/v1/rooms/{code}/join` | сессия |
| `DELETE` | `/v1/rooms/{code}/members/me` | сессия |
| `DELETE` | `/v1/rooms/{code}` | владелец |
| `GET` | `/v1/discover/groups?q=&limit=&offset=` | нет |
| `GET` | `/healthz`, `/readyz` | нет |

Коды временных комнат имеют 26 символов из безопасного 32-символьного алфавита
(130 бит). Миграция безопасности удаляет старые короткие комнаты; активные
комнаты придётся создать заново после обновления.

Discover принимает `limit=1..50`, `offset=0..1000`, а непустой поиск — от 2 до
80 Unicode-символов. Аватар пользователя сначала проверяется через
`image.DecodeConfig`, ограничивается 4096×4096 и 4 млн пикселей, затем
декодируется и сохраняется как ограниченный JPEG. Аватар группы — только короткий
символ/emoji, не data URL.

## Curl-сценарий

```bash
COOKIE_JAR=/tmp/threaden-cookies.txt
curl -sS -c "$COOKIE_JAR" -X POST http://localhost:8080/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"gleb@example.com","password":"password123"}'

curl -sS -b "$COOKIE_JAR" -X POST http://localhost:8080/v1/rooms
curl -i -b "$COOKIE_JAR" -X DELETE http://localhost:8080/v1/auth/logout
```

## Проверки

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
docker compose config
```
