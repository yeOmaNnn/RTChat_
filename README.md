Пет-проект реализует, WebSocket-чат с комнатами, историей сообщений в PostgreSQL,
hub-паттерном на горутинах и каналах, rate limiting и graceful shutdown.
Опционально — Redis pub/sub для горизонтального масштабирования на несколько
инстансов приложения.

## Возможности

- Комнаты создаются "по требованию" при первом подключении (`?room=<id>`)
- История сообщений хранится в PostgreSQL и отдаётся при входе в комнату
- Rate limiting на сообщения (token bucket, настраиваемый)
- Graceful shutdown по SIGINT/SIGTERM — сервер корректно закрывает HTTP,
  WebSocket-соединения и пул БД перед выходом
- Опциональный Redis pub/sub — несколько реплик приложения видят сообщения
  друг друга через общую шину

## Стек

Go 1.22+ · `gorilla/websocket` · `pgx/v5` (PostgreSQL) · `go-redis/v9` ·
`golang.org/x/time/rate` · Docker / Docker Compose

## Структура проекта

```
chatapp/
├── cmd/server/main.go          
├── internal/
│   ├── config/config.go       
│   ├── storage/               
│   │   ├── models.go
│   │   └── postgres.go
│   ├── ratelimit/limiter.go    
│   ├── pubsub/redis.go         
│   ├── ws/                      
│   │   ├── client.go            
│   │   ├── room.go            
│   │   └── hub.go              
│   └── httpapi/handlers.go      
├── migrations/001_init.sql
├── Dockerfile
├── docker-compose.yml
├── .env.example
└── go.mod
```

## Архитектура

### Hub-паттерн: Hub → Room → Client

- **`Hub`** — потокобезопасный реестр `map[roomID]*Room`. Комната создаётся
  лениво при первом подключении.
- **`Room`** — хаб одной комнаты. Работает в единственной горутине `run()`,
  которая в одном `select` слушает каналы `register` / `unregister` / `incoming`.
- **`Client`** — обёртка над одним `*websocket.Conn`, две горутины:
  `ReadPump` (чтение + rate limit) и `WritePump` (**единственный** писатель
  в сокет — это требование gorilla/websocket, плюс периодические `ping`).

### Путь сообщения

1. Клиент → `ReadPump` → проверка rate limit → канал `room.incoming`
2. `Room.handleIncoming`: сначала пишет в Postgres, только потом рассылает
   локальным клиентам
3. Если настроен Redis — публикует в канал `chat:room:<id>`, другие инстансы
   получают его через подписку и рассылают своим локальным клиентам

### Rate limiting

Token bucket (`golang.org/x/time/rate`) на клиента, ключ — уникальный uuid
соединения. По умолчанию 5 сообщений/сек, burst 10. Превышение — сообщение
отбрасывается, соединение не закрывается.

### Graceful shutdown

`SIGINT`/`SIGTERM` → `httpServer.Shutdown()` (перестаём принимать новых) →
`hub.Shutdown()` (закрываем открытые WS, ждём завершения горутин всех комнат) →
закрытие пула Postgres / клиента Redis.

## Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `HTTP_ADDR` | `:8080` | адрес HTTP/WS сервера |
| `POSTGRES_DSN` | `...` | строка подключения к Postgres |
| `REDIS_ADDR` | *(пусто)* | адрес Redis; пусто = pub/sub выключен, режим одного инстанса |
| `RATE_LIMIT_RPS` | `5` | сообщений в секунду на клиента |
| `HISTORY_LIMIT` | `50` | сколько последних сообщений отдавать при входе в комнату |
| `SHUTDOWN_TIMEOUT_SEC` | `10` | таймаут shutdown в секундах |

## Запуск

Требуется только Docker и Docker Compose.

1. Проверьте свою версию Go (`go version`) и убедитесь, что в `Dockerfile`
   тег образа `golang:X.YY-alpine` совпадает или новее — иначе сборка упадёт
   с ошибкой вида `go.mod requires go >= 1.25.0 (running go 1.22.12)`.

2. Сгенерируйте `go.sum` (один раз, локально, нужен интернет):
```bash
   go mod tidy
```

3. Поднимите всё:
```bash
   docker compose up --build
```
   Поднимутся Postgres (миграция `001_init.sql` применится автоматически
   при первом старте), Redis и само приложение на `localhost:8080`.

4. Проверка:
```bash
   curl http://localhost:8080/healthz
   # ok
```

Остановка:
```bash
docker compose down        # с сохранением данных Postgres
docker compose down -v     # + удалить volume с данными
```

### Локально, без Docker

Нужен установленный Go 1.22+ и работающий Postgres (Redis опционален).

```bash
# зависимости
go mod tidy

# Postgres быстрым способом через Docker (если ещё не установлен)
docker run -d --name chat-postgres \
  -e POSTGRES_USER=chat -e POSTGRES_PASSWORD=chat -e POSTGRES_DB=chat \
  -p 5432:5432 postgres:16-alpine

# применяем миграцию
docker exec -i chat-postgres psql -U chat -d chat < migrations/001_init.sql

# конфиг
cp .env.example .env

# запуск
go run ./cmd/server
```

## Тестирование вручную

### WebSocket

Через [`websocat`](https://github.com/vi/websocat) или `wscat` (`npm i -g wscat`),
в двух терминалах:

```bash
websocat "ws://localhost:8080/ws?room=general&username=vasya"
websocat "ws://localhost:8080/ws?room=general&username=petya"
```

При подключении прилетит история комнаты (`{"history":[...]}`). Отправьте
в любом терминале:

```json
{"content": "привет всем"}
```

Сообщение должно появиться в обоих терминалах.

### Rate limiting

Быстро отправьте >10 сообщений подряд в одном сеансе — после исчерпания burst
начнут приходить `{"system": "слишком много сообщений, притормози"}`, а само
сообщение в чат не попадёт.

### История через REST

```bash
curl http://localhost:8080/api/rooms/general/history
```

### Graceful shutdown

Держите открытым WS-соединение и остановите сервер (`Ctrl+C` или
`docker compose stop app`) — в логах будет видна последовательность остановки,
а у клиента соединение закроется штатным close-фреймом, а не обрывом.
