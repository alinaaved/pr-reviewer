.PHONY: up down run logs test lint

up:
	docker compose -f deploy/docker-compose.yml up -d --build

down:
	docker compose -f deploy/docker-compose.yml down -v

run:
	APP_PORT=${APP_PORT:-:8080} DB_DSN=$(awk -F= '/^DB_DSN=/{print $$2}' configs/.env.example) go run ./cmd/server

logs:
	docker compose -f deploy/docker-compose.yml logs -f app

test:
	go test ./... -v

lint:
	golangci-lint run -v --config .golangci.yml