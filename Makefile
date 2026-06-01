.PHONY: dev-backend dev-frontend build test

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
