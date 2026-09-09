.PHONY: build run run-worker test integration-test fmt vet check docker-up docker-down kafka-topics kafka-groups db-shell db-events

build:
	go build ./...

run:
	go run ./cmd/gateway

run-worker:
	go run ./cmd/worker

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

kafka-groups:
	docker compose exec kafka /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --group telemetryforge-processors

db-shell:
	docker compose exec timescaledb psql -U telemetryforge -d telemetryforge

db-events:
	docker compose exec timescaledb psql -U telemetryforge -d telemetryforge -c "SELECT event_id, source, event_type, event_time FROM telemetry_events ORDER BY event_time DESC LIMIT 20;"
