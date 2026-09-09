.PHONY: build run test integration-test fmt vet check docker-up docker-down kafka-topics

build:
	go build ./...

run:
	go run ./cmd/gateway

test:
	go test ./...

integration-test:
	TELEMETRYFORGE_INTEGRATION_KAFKA_BROKERS=localhost:9092 go test -count=1 ./tests/integration

fmt:
	gofmt -w cmd internal tests

vet:
	go vet ./...

check: fmt vet test build

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v

kafka-topics:
	docker compose exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list
