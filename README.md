# Booking Service

Веб-сервис на Go, моделирующий процесс бронирования переговорки через машину состояний.  


---

## 1. Технологии

| Технология | Назначение |
|---|---|
| **[Go 1.22](https://go.dev/)** | Язык разработки |
| **[Fiber v2](https://gofiber.io/)** | HTTP-фреймворк |
| **[Docker](https://www.docker.com/)** | Контейнеризация |

---

## 2. Структура проекта

```
frameworks_4/
├── main.go                        
├── handlers.go                    
├── go.mod                         
├── Dockerfile                     
├── docker-compose.yml             
├── booking-service.postman_collection.json
└── internal/
    ├── state/
    │   └── machine.go             
    ├── metrics/
    │   └── metrics.go             
    └── health/
        └── health.go              
```



## 3. Первый запуск


### Клонирование и запуск

```bash
git clone https://github.com/aquamarine27/frameworks_4.git
cd frameworks_4
docker compose up --build
```

Сервер будет доступен на `http://localhost:3000`.

### Проверка

```bash
# Liveness
curl http://localhost:3000/healthz/live

# Readiness
curl http://localhost:3000/healthz/ready

# Метрики
curl http://localhost:3000/metrics
```



## 4. API

| Метод | Путь | Описание |
|---|---|---|
| `POST` | `/event` | Отправить событие процессу |
| `GET` | `/process/:key` | Получить состояние процесса |
| `GET` | `/processes` | Список всех процессов |
| `GET` | `/healthz/live` | Liveness probe |
| `GET` | `/healthz/ready` | Readiness probe |
| `GET` | `/metrics` | Метрики в plain text |
| `POST` | `/admin/degrade` | Включить/выключить критическую деградацию |

### Пример запроса

```bash
curl -X POST http://localhost:3000/event \
  -H "Content-Type: application/json" \
  -d '{
    "process_key":     "room-101-2025-06-01",
    "idempotency_key": "evt-001",
    "correlation_id":  "corr-abc",
    "event":           "ПринятьЗаявку"
  }'
```

```json
{
  "correlation_id": "corr-abc",
  "process_key":    "room-101-2025-06-01",
  "status":         "ok",
  "prev_state":     "Новый",
  "next_state":     "ЗаявкаПринята"
}
```