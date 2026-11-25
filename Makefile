SERVICE := postgres
DB := bank
TEST_DB := bank_test

.PHONY: db-up db-down db-reset wait-db migrate-up migrate-test test run

db-up:
	docker compose up -d $(SERVICE)

db-down:
	docker compose down

db-reset:
	docker compose down -v

wait-db: db-up
	@until docker compose exec -T $(SERVICE) pg_isready -U postgres -d $(DB) >/dev/null 2>&1; do sleep 1; done

migrate-up: wait-db
	docker compose exec -T $(SERVICE) psql -v ON_ERROR_STOP=1 -U postgres -d $(DB) < migrations/init.sql

migrate-test: wait-db
	@docker compose exec -T $(SERVICE) psql -U postgres -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$(TEST_DB)'" | grep -q 1 || docker compose exec -T $(SERVICE) createdb -U postgres $(TEST_DB)
	docker compose exec -T $(SERVICE) psql -v ON_ERROR_STOP=1 -U postgres -d $(TEST_DB) < migrations/init.sql

test: migrate-test
	go test ./... -count=1

run: migrate-up
	go run .
