# Threaden backend

Версия: 0.3.3

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

Локальный compose ограничивает LiveKit signaling адресом `127.0.0.1`, но для
WebRTC объявляет доступные интерфейсы хоста и loopback, а также запускает
встроенный TURN/UDP на порту `3478` выбранного адреса хоста. Это необходимо
Firefox и браузерам с
ограниченными host-кандидатами: одного loopback ICE-кандидата для медиасоединения
недостаточно. TURN в dev-конфигурации разрешает relay только к loopback и
приватным сетям, где находится локальный SFU.
Для подключения с другого устройства используйте production-конфигурацию из
`deploy/livekit.yaml` с публичным IP и TURN, а не этот dev-стенд.

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
- хранение только SHA-256 хеша токена в SQLite;
- управление паролем и списком активных сеансов только после 24 часов с момента входа.

Пароль должен содержать от 6 до 72 символов (и не превышать технический предел bcrypt в 72 байта). Пароли хранятся как bcrypt-хеши.
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
| `TRUSTED_PROXIES` | Разделённые запятыми IP/CIDR доверенных непосредственных proxy; без значения forwarding-заголовки игнорируются |
| `RATE_LIMIT_BUCKET_TTL` | TTL persistent rate-limit bucket |
| `MAX_USER_GROUPS` | Максимум групп, которыми может владеть аккаунт |
| `DISCOVER_MIN_MEMBERS` | Минимум участников для показа в Discover |
| `IP_BAN_THRESHOLD` | Число rate-limit-нарушений с одного IP до бана |
| `IP_BAN_WINDOW` | Окно, в котором считаются нарушения |
| `IP_BAN_STEPS` | Длительности бана по уровням эскалации через запятую |
| `IP_BAN_ESCALATION_FORGET` | Период без нарушений, после которого уровень эскалации сбрасывается |
| `ACCOUNT_BAN_WINDOW` | Окно учёта банов максимального уровня для авто-удаления аккаунта |
| `ACCOUNT_BAN_DELETION_COUNT` | Число банов максимального уровня за окно до авто-удаления аккаунта |
| `ATTACHMENT_STORAGE_DIR` | Каталог бинарных вложений |
| `ATTACHMENT_MAX_INPUT_MEDIA_BYTES` | Максимальный входной размер медиа |
| `ATTACHMENT_MAX_ARCHIVE_BYTES` | Максимальный размер архива |
| `ATTACHMENT_MAX_OUTPUT_MEDIA_BYTES` | Максимальный размер обработанного медиа |
| `ATTACHMENT_MAX_FILES_PER_MESSAGE` | Максимум файлов в сообщении |
| `ATTACHMENT_MAX_USER_STORED_BYTES` | Активная квота пользователя |
| `ATTACHMENT_MAX_USER_DAILY_BYTES` | Суточный лимит новых вложений |
| `ATTACHMENT_MAX_TOTAL_BYTES` | Общая квота вложений |
| `ATTACHMENT_RETENTION` | Срок хранения вложения |

Остальные cleanup- и anti-spam-переменные перечислены в `.env.example`.

`TRUSTED_PROXIES` должен содержать только конкретные адреса или узкие CIDR
ваших reverse proxy (например, `127.0.0.1,::1`). Backend принимает
`X-Forwarded-For` только от такого непосредственного peer, разбирает цепочку
справа налево и при некорректной цепочке использует TCP peer. Нельзя указывать
`0.0.0.0/0` или `::/0`.

Изменение профиля и удаление аватара ограничены до одного действия раз в 3 минуты;
создание групп — до одной группы раз в 3 минуты, а владение — до `MAX_USER_GROUPS`
групп на аккаунт. Discover показывает только публичные группы с числом участников
не ниже `DISCOVER_MIN_MEMBERS`. Попытка сохранить профиль без изменений отклоняется
сервером и не записывается в базу данных. IP, повторивший rate-limit-нарушение
`IP_BAN_THRESHOLD` раз в течение `IP_BAN_WINDOW`, автоматически блокируется.
Длительность бана выбирается по уровню эскалации из `IP_BAN_STEPS`
(10 секунд → минута → 5 минут → 24 часа); после `IP_BAN_ESCALATION_FORGET`
без нарушений уровень сбрасывается. Если бан максимального уровня повторяется
`ACCOUNT_BAN_DELETION_COUNT` раз в течение `ACCOUNT_BAN_WINDOW`, аккаунт
запроса, вызвавшего такой бан, автоматически удаляется.

Группа может быть временно изолирована при атаке массовыми вступлениями или
сообщениями. Изоляция не удаляет группу и не ограничивает владельца или текущих
участников: она блокирует только новые public/invite-вступления. В API состояние
отдаётся как `join_blocked` и `join_blocked_until`; попытка вступления в этот
период возвращает HTTP 423 с кодом `group_isolated` и `Retry-After`.

## HTTP API

| Метод | Endpoint | Авторизация |
|---|---|---|
| `POST` | `/v1/auth/register` | нет |
| `POST` | `/v1/auth/login` | нет |
| `DELETE` | `/v1/auth/logout` | cookie или Bearer; идемпотентно |
| `GET` | `/v1/me` | сессия |
| `GET` | `/v1/welcome` | сессия; сводка активности за последние 24 часа |
| `PATCH` | `/v1/me` | сессия, multipart avatar |
| `DELETE` | `/v1/me/avatar` | сессия |
| `DELETE` | `/v1/me` | сессия |
| `POST` | `/v1/rooms` | сессия |
| `GET` | `/v1/rooms/{code}` | сессия |
| `POST` | `/v1/rooms/{code}/join` | сессия |
| `DELETE` | `/v1/rooms/{code}/members/me` | сессия |
| `DELETE` | `/v1/rooms/{code}` | владелец |
| `GET` | `/v1/groups/{id}/messages` | preview public-группы; private — только участнику, с момента вступления |
| `POST` | `/v1/groups/{id}/messages` | сессия; поддерживает `reply_to_id` |
| `GET` | `/v1/attachments/{id}` | участник группы |
| `DELETE` | `/v1/groups/{id}/messages/{messageID}` | автор сообщения или владелец группы |
| `GET` | `/v1/groups/{id}/profile` | сессия; public — без членства, private — только участнику |
| `GET` | `/v1/discover/groups?q=&limit=&offset=` | нет |
| `GET` | `/healthz`, `/readyz` | нет |

`/v1/welcome` можно запрашивать в любое время. Backend пересчитывает общий снимок
статистики не чаще одного раза в час, а web-клиент кэширует полученный ответ на
час в текущей сессии браузера.

Коды временных комнат имеют 26 символов из безопасного 32-символьного алфавита
(130 бит). Миграция безопасности удаляет старые короткие комнаты; активные
комнаты придётся создать заново после обновления.

Discover принимает `limit=1..50`, `offset=0..1000`, а непустой поиск — от 2 до
80 Unicode-символов. Аватар пользователя сначала проверяется через
`image.DecodeConfig`, ограничивается 4096×4096 и 4 млн пикселей, затем
декодируется и сохраняется как ограниченный JPEG. Аватар группы — только короткий
символ/emoji, не data URL.

Для обработки видео production-окружение должно иметь установленный `ffmpeg`.
Вложения проходят проверку содержимого, а не расширения; неизвестные форматы
отклоняются.

Сообщение можно отправить JSON-ом как раньше или `multipart/form-data` с полем
`body` и одним-тремя полями `files[]`. Подпись необязательна. Фото и видео после
проверки перекодируются до 1 МБ, архивы принимаются по сигнатуре до 5 МБ и не
распаковываются. Активная квота пользователя — 50 МБ, срок хранения — 72 часа.

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
