.PHONY: build run test fmt vet check docker-up docker-down

build:
	go build ./...

run:
	go run ./cmd/gateway

test:
	go test ./...

fmt:
	gofmt -w cmd internal

vet:
	go vet ./...

check: fmt vet test build

docker-up:
	docker compose up --build

docker-down:
	docker compose down
