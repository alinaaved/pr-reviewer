# PR Reviewer Assignment Service

Сервис для автоматического назначения ревьюверов на Pull Request, управления командами и активностью пользователей

- Язык: Go  
- Web: chi  
- ORM: GORM  
- БД: PostgreSQL  
- Запуск: `docker compose up` (поднимает БД + приложение на `:8080`)

## Как запустить

### через docker compose (поднимет БД и сервис на 8080)
```
make up
curl localhost:8080/healthz   # -> {"status":"ok"}
```

### локально (если БД уже поднята compose-ом на 5432)
```
make run
```

## Как остановить
```
make down
```

## Переменные окружения

`APP_PORT` — порт HTTP-сервера (по умолчанию :8080)
`DB_DSN` — строка подключения к PostgreSQL
локально: `postgres://app:app@localhost:5432/app?sslmode=disable`
в docker-compose: `postgres://app:app@db:5432/app?sslmode=disable`

Шаблон: configs/.env.example.
.env в git не коммитится (см. .gitignore).

## Makefile

make up    # поднять БД и приложение (compose up -d --build)
make down  # остановить и удалить контейнеры и том БД
make run   # локальный запуск (читает DB_DSN из .env.example)
make logs  # логи приложения (docker compose logs -f app)
make test  # go test ./... -v
make lint  # golangci-lint run

## Архитектура данных

```
teams(team_name PK)

users(
  user_id PK,
  username,
  is_active,
  team_name FK -> teams(team_name)
)

pull_requests(
  pull_request_id PK,
  pull_request_name,
  author_id FK -> users(user_id),
  status CHECK ('OPEN'|'MERGED') DEFAULT 'OPEN',
  created_at timestamptz DEFAULT now(),
  merged_at  timestamptz NULL
)

pr_reviewers(
  pr_id FK -> pull_requests(pull_request_id),
  reviewer_id FK -> users(user_id),
  position SMALLINT CHECK (position in (1,2)),
  PRIMARY KEY (pr_id, position),
  UNIQUE (pr_id, reviewer_id)
)

-- индексы
CREATE INDEX idx_pr_reviewers_reviewer ON pr_reviewers(reviewer_id);
CREATE INDEX idx_pr_status ON pull_requests(status);
```

- Назначенных ревьюверов храним в отдельной таблице с позициями `1/2` (0..2 ревьювера).

## Доменные правила

- **Создание PR**: автоматически назначаются до **2** активных ревьюверов из **команды автора**; автора не назначаем; если активных меньше — назначаем 0/1.  
- **Переназначение**: заменяем одного ревьювера на **случайного активного** из **команды заменяемого**; не назначаем автора и второго текущего.  
- **После MERGED** менять ревьюверов нельзя (`409 PR_MERGED`).  
- **Merge** — идемпотентен (повторный вызов возвращает актуальное состояние).

## Маршруты

- `POST /team/add` — создать команду и **upsert** участников (повтор по контракту: `400 TEAM_EXISTS`)
- `GET /team/get?team_name=...` — получить команду и участников
- `POST /users/setIsActive` — переключить активность пользователя
- `GET /users/getReview?user_id=...` — список PR, где пользователь назначен ревьювером
- `POST /pullRequest/create` — создать PR и автоназначить ревьюверов
- `POST /pullRequest/merge` — пометить PR `MERGED` (идемпотентно)
- `POST /pullRequest/reassign` — переназначить конкретного ревьювера
- `GET /healthz` — liveness
- `GET /stats/assignments-by-user` — простая статистика назначений по пользователям

## Примеры запросов (curl)

```
# создать команду с участниками (upsert пользователей)
curl -X POST localhost:8080/team/add -H 'Content-Type: application/json' -d '{
  "team_name":"backend",
  "members":[
    {"user_id":"u1","username":"Alice","is_active":true},
    {"user_id":"u2","username":"Bob","is_active":true},
    {"user_id":"u3","username":"Carol","is_active":true},
    {"user_id":"u4","username":"Dave","is_active":true}
  ]
}'

# получить команду
curl 'localhost:8080/team/get?team_name=backend'

# создать PR (автоназначение до 2 ревьюверов из команды автора, кроме автора и неактивных)
curl -X POST localhost:8080/pullRequest/create -H 'Content-Type: application/json' -d '{
  "pull_request_id":"pr-2001",
  "pull_request_name":"Feature A",
  "author_id":"u1"
}'

# переназначить одного ревьювера на случайного активного из его команды
curl -X POST localhost:8080/pullRequest/reassign -H 'Content-Type: application/json' -d '{
  "pull_request_id":"pr-2001",
  "old_user_id":"u2"
}'

# пометить PR как MERGED (идемпотентно)
curl -X POST localhost:8080/pullRequest/merge -H 'Content-Type: application/json' -d '{
  "pull_request_id":"pr-2001"
}'

# список PR, где пользователь назначен ревьювером
curl 'localhost:8080/users/getReview?user_id=u3'
```

## Нагрузочное тестирование
Инструмент: k6, 5 VU, 20s

Команда: `BASE_URL=http://localhost:8080 k6 run k6/script.js`

Результаты на локальной машине (Docker):
- Всего запросов: ~941 (~47 rps)
- Задержка p95: ~15 ms
- Ошибки: 0% (после проверки существования команды в setup)

## Тесты, линтер
```
go test ./... -v
make lint
```


## Структура репозитория

```
.
├── cmd/server/main.go
├── internal/
│   ├── http/            # httpapi: DTO + handlers
│   └── model/           # GORM-модели (маппинг на таблицы)
├── db/migrations/001_init.sql
├── deploy/docker-compose.yml
├── openapi.yml
├── Makefile
├── configs/.env.example
└── k6/script.js         # сценарий нагрузки
```
