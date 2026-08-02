# Управление Threaden через systemd

В корне проекта находится `threadenctl.sh`. Он создаёт три независимых unit:

- `threaden-backend.service` — собранный Go API;
- `threaden-web.service` — production-сборка Angular, которую обслуживает отдельный
  nginx-процесс;
- `threaden-livekit.service` — локальный LiveKit в Docker с host networking.

Скрипт предназначен для Linux-сервера с systemd. По умолчанию web слушает
`127.0.0.1:18081`, backend — `127.0.0.1:18080`. Публичный TLS reverse proxy
должен проксировать сайт на `127.0.0.1:18081`. Значения можно изменить после
первого запуска в `/etc/threaden/threadenctl.conf`.

## Перед первым запуском

Создайте production-конфигурацию и замените все заглушки:

```bash
cp deploy/production.env.example deploy/production.env
chmod 600 deploy/production.env
```

В `deploy/livekit.yaml` укажите тот же `LIVEKIT_API_KEY` и
`LIVEKIT_API_SECRET`, что и в `deploy/production.env`. Настройте TURN-домен и
пути к сертификатам либо отключите TURN до появления сертификатов.

Для HTTPS deployment оставьте:

```dotenv
SESSION_COOKIE_SECURE=true
CORS_ALLOWED_ORIGINS=https://ваш-домен.example
TRUSTED_PROXIES=127.0.0.1,::1
```

## Команды

```bash
sudo ./threadenctl.sh doctor
sudo ./threadenctl.sh start
sudo ./threadenctl.sh restart
sudo ./threadenctl.sh restart --full
sudo ./threadenctl.sh stop
sudo ./threadenctl.sh recovery --yes
sudo ./threadenctl.sh status
sudo ./threadenctl.sh logs
```

Управление отдельными компонентами:

```bash
sudo ./threadenctl.sh start --backend
sudo ./threadenctl.sh restart --web
sudo ./threadenctl.sh restart --full --backend --web
sudo ./threadenctl.sh recovery --livekit --yes
sudo ./threadenctl.sh stop --backend --web
```

Флаги можно комбинировать. Без флагов выбираются backend, web и LiveKit.
Если используется внешний LiveKit, запускайте `--backend --web`, не выбирая
`--livekit`.

## Что делает recovery

`recovery`:

1. при поддерживаемом package manager устанавливает недостающие runtime-пакеты;
2. останавливает и удаляет только Threaden unit-файлы;
3. завершает процессы из Threaden systemd-cgroup и контейнер
   `threaden-livekit`;
4. очищает только `/var/cache/threaden-*`;
5. исправляет владельца и режим runtime-файлов;
6. пересобирает backend и web;
7. заново проверяет nginx и systemd unit-файлы;
8. запускает сервисы в порядке LiveKit → backend → web и ждёт health checks.

База `/var/lib/threaden/app.db` не удаляется. Скрипт не выполняет `chmod -R
777`, не сбрасывает kernel page cache, не вызывает глобальный `docker system
prune` и не убивает случайный процесс только потому, что тот занял похожий
порт.

## Сборка без Go или Node на хосте

Backend собирается локальным Go 1.26+, если он доступен; иначе используется
контейнер `golang:1.26-alpine`. Web собирается Node.js 24+; при его отсутствии
используется `node:24-alpine`. Для fallback-сборки и локального LiveKit нужен
работающий Docker daemon.

## Публичный reverse proxy

Минимальный внешний nginx location:

```nginx
location / {
    proxy_pass http://127.0.0.1:18081;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto https;
    proxy_buffering off;
    proxy_read_timeout 3600s;
}
```

TLS, сертификаты и публичные домены остаются ответственностью внешнего reverse
proxy. Не выставляйте backend-порт `18080` напрямую в интернет.
