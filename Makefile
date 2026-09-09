.PHONY: build run run-worker telemetryctl dashboard-dev dashboard-build demo-traffic demo-incident demo-cardinality test integration-test fmt vet check docs-check policy-check k8s-render terraform-check docker-up docker-down kafka-topics kafka-groups db-shell db-events dlq-tail

build:
	go build ./...

run:
	go run ./cmd/gateway

run-worker:
	go run ./cmd/worker

telemetryctl:
	go run ./cmd/telemetryctl

dashboard-dev:
	cd dashboard && npm install --no-audit --no-fund && npm run dev

dashboard-build:
	cd dashboard && npm install --no-audit --no-fund && npm run lint && npm run build

test:
	go test ./...

integration-test:
	TELEMETRYFORGE_INTEGRATION_KAFKA_BROKERS=localhost:9092 \
	TELEMETRYFORGE_INTEGRATION_DATABASE_URL=postgres://telemetryforge:telemetryforge@localhost:5432/telemetryforge?sslmode=disable \
	go test -count=1 ./tests/integration

fmt:
	gofmt -w cmd internal tests

vet:
	go vet ./...

check: fmt vet test build

docs-check:
	python3 scripts/check-docs.py
	git diff --check
	docker compose config --quiet


policy-check:
	go run ./cmd/telemetryctl policy validate --file policies/active.json
	go run ./cmd/telemetryctl policy validate --file policies/shadow.json

k8s-render:
	kubectl kustomize deployments/kubernetes/base >/dev/null

terraform-check:
	terraform -chdir=infra/terraform/aws fmt -check
	terraform -chdir=infra/terraform/aws init -backend=false -input=false
	terraform -chdir=infra/terraform/aws validate

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

dlq-tail:
	docker compose exec kafka /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic telemetry.dlq --from-beginning

demo-traffic:
	python3 scripts/demo-traffic.py

demo-incident:
	python3 scripts/demo-traffic.py --incident

demo-cardinality:
	python3 scripts/demo-traffic.py --cardinality --count 160 --interval 0.05
