.PHONY: dev-backend dev-frontend build test docker-build docker-run docker-up

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

build:
	cd frontend && npm run build
	cd backend && go build -o bin/ludex ./cmd/server

test:
	cd backend && go test ./...
	cd frontend && npm run build

docker-build:
	docker build -t ludex:local .

docker-run:
	docker run --rm -p 8787:8787 -v ludex-data:/data ludex:local

docker-up:
	docker compose up --build
