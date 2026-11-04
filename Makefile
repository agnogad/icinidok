.PHONY: help dev-api dev-web test build docker up down clean

help:
	@echo "make dev-api   # Go API :8080"
	@echo "make dev-web   # Next.js :3000 (API'ye 8080'den bağlanır)"
	@echo "make test      # Go testleri"
	@echo "make docker    # imaj derle"
	@echo "make up        # docker compose up"
	@echo "make down      # docker compose down"

dev-api:
	cd services/api && go run ./cmd/server

dev-web:
	cd apps/web && npm run dev

test:
	cd services/api && go test ./...

docker:
	docker build -t icinidok:local .

up:
	docker compose up --build -d

down:
	docker compose down

clean:
	rm -rf services/api/data apps/web/.next apps/web/out
