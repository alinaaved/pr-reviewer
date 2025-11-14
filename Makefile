compose-up:
	docker compose -f deploy/docker-compose.yml up -d

compose-down:
	docker compose -f deploy/docker-compose.yml down -v

run:
	DB_DSN=$$(grep DB_DSN configs/.env.example|cut -d= -f2-) go run ./cmd/server