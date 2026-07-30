# Production deployment

Для Linux/systemd основной вариант управления описан в
[`SYSTEMD.md`](SYSTEMD.md). Скрипт `../threadenctl.sh` собирает backend и web,
создаёт отдельные unit-файлы и при необходимости управляет локальным LiveKit.

1. Скопируйте безопасный шаблон и заполните секреты вне Git:

```bash
cp deploy/production.env.example deploy/production.env
chmod 600 deploy/production.env
```

`LIVEKIT_API_KEY` и `LIVEKIT_API_SECRET` должны быть случайными production-
значениями. Никогда не коммитьте `deploy/production.env`. При подозрении на
утечку замените оба значения.

2. В production обязательно установите:

```dotenv
SESSION_COOKIE_SECURE=true
CORS_ALLOWED_ORIGINS=https://threaden.example.com
```

Frontend лучше обслуживать на том же origin, где nginx проксирует `/v1/` в API.
Это упрощает host-only cookie и исключает ненужный credentialed CORS.

3. Перед запуском синхронизируйте LiveKit key/secret с `deploy/livekit.yaml` или
сгенерируйте конфиг из секретного хранилища. Файл в репозитории содержит только
заглушки.

```bash
cd web-client
npm ci
npm run build
sudo rsync -a --delete dist/ /var/www/threaden/

cd ..
docker compose -f docker-compose.prod.yml up --build -d
docker compose -f docker-compose.prod.yml logs -f
```

Reverse proxy должен сохранять nginx rate/connection limits, задавать
`X-Forwarded-For`, завершать TLS и проксировать SSE без buffering. Backend
должен слушать loopback, а не публичный интерфейс. Для volumetric DDoS всё равно
нужны firewall/CDN/provider-level controls.

После обновления до migration 9 старые четырёхсимвольные временные комнаты
удаляются, поскольку они являлись перебираемым секретом.
