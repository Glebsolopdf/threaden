# Threaden: готовая production-конфигурация

Домены уже настроены в архиве:

- сайт: `threaden.substituteme.space`
- API: `api.substituteme.space`
- LiveKit: `livekit.substituteme.space`
- TURN: `turn.substituteme.space`
- редирект: `opentalk.substituteme.space` → `threaden.substituteme.space`

В `deploy/production.env` и `deploy/livekit.yaml` находятся одинаковые случайно
сгенерированные LiveKit credentials. Эти файлы являются секретными. Не публикуйте
архив и не коммитьте их в Git.

## Установка

```bash
chmod 600 deploy/production.env
sudo ./threadenctl.sh doctor
sudo ./threadenctl.sh start --yes
```

Затем установите внешний nginx-конфиг:

```bash
sudo cp deploy/nginx.conf /etc/nginx/conf.d/threaden.conf
sudo nginx -t
sudo systemctl reload nginx
```

Если основной `/etc/nginx/nginx.conf` не включает `/etc/nginx/conf.d/*.conf`,
подключите файл вручную.

## Сетевые порты

Для сайта/API/LiveKit HTTPS откройте TCP `80` и `443`.
Для WebRTC/LiveKit откройте:

- TCP `7881`
- UDP `3478`
- TCP `5349`
- UDP `50000:50100`

TURN TLS читает сертификаты из:

- `/etc/letsencrypt/live/turn.substituteme.space/fullchain.pem`
- `/etc/letsencrypt/live/turn.substituteme.space/privkey.pem`

Проверьте существование этих путей перед `threadenctl.sh start`.
