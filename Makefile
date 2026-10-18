.PHONY: all run build test lint vuln docker-up docker-down

all: build

run:
	go run ./cmd/server

build:
	go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

test:
	go test -race ./...

lint:
	golangci-lint run ./...

vuln:
	govulncheck ./...

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
