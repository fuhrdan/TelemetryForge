.PHONY: build run run-edge run-worker run-router routing-check telemetryctl dashboard-dev dashboard-build demo-traffic demo-incident demo-cardinality demo-evidence demo-schema demo-routing test integration-test fmt vet check docs-check policy-check k8s-render terraform-check docker-up docker-down kafka-topics kafka-groups db-shell db-events dlq-tail load-smoke load-sustained load-backpressure observability-check demo-shaping archive-check demo-change connector-check proof-check proof-verify proof-k6-smoke proof-k6-sustained proof-k6-backpressure proof-kafka-outage proof-postgres-outage proof-worker-failover proof-connector-outage proof-backup-restore proof-wal-crash-recovery proof-edge-replication proof-mesh-failover proof-cryptographic-lineage proof-formal-verification proof-fastpath-performance proof-autonomous-control formal-check edge-check intelligence-check autonomy-check plugin-check

build:
	go build ./...

run:
	go run ./cmd/gateway

run-edge:
	go run ./cmd/edge

run-worker:
	go run ./cmd/worker

run-router:
	go run ./cmd/router

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

routing-check:
	go run ./cmd/telemetryctl routing validate --file routing/active.json
	go run ./cmd/telemetryctl routing validate --file routing/shadow.json

k8s-render:
	kubectl kustomize deployments/kubernetes/base >/dev/null
	kubectl kustomize deployments/kubernetes/overlays/production >/dev/null

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

demo-evidence:
	python3 scripts/demo-traffic.py --evidence-demo --count 45 --interval 0.20

demo-schema:
	python3 scripts/demo-traffic.py --schema-demo --interval 0.05

demo-routing:
	python3 scripts/demo-traffic.py --routing-demo --count 80 --interval 0.08

# Mix healthy requests with protected errors/high latency and large payloads.
demo-shaping:
	python3 scripts/demo-traffic.py --shaping-demo --count 140 --interval 0.04

observability-check:
	docker run --rm --entrypoint /bin/promtool \
		-v "$(CURDIR)/deployments/observability/prometheus:/etc/prometheus:ro" \
		prom/prometheus:v3.13.3 check config /etc/prometheus/prometheus.yml
	docker run --rm \
		-v "$(CURDIR)/deployments/observability/otel/collector.yaml:/etc/otelcol/config.yaml:ro" \
		otel/opentelemetry-collector:0.160.0 \
		validate --config=/etc/otelcol/config.yaml
	docker run --rm \
		-v "$(CURDIR)/deployments/observability/tempo/tempo.yaml:/etc/tempo/tempo.yaml:ro" \
		grafana/tempo:3.0.2 \
		-config.file=/etc/tempo/tempo.yaml -config.verify
	python3 -c 'import json, pathlib; [json.loads(p.read_text()) for p in pathlib.Path("deployments/observability/grafana/dashboards").glob("*.json")]'

load-smoke:
	docker run --rm --add-host host.docker.internal:host-gateway \
		-v "$(CURDIR)/load/k6:/scripts:ro" \
		grafana/k6:2.2.0 run /scripts/ingest-smoke.js

load-sustained:
	docker run --rm --add-host host.docker.internal:host-gateway \
		$(if $(RATE),-e RATE="$(RATE)") $(if $(DURATION),-e DURATION="$(DURATION)") \
		-v "$(CURDIR)/load/k6:/scripts:ro" \
		grafana/k6:2.2.0 run /scripts/ingest-sustained.js

load-backpressure:
	docker run --rm --add-host host.docker.internal:host-gateway \
		$(if $(RATE),-e RATE="$(RATE)") $(if $(DURATION),-e DURATION="$(DURATION)") \
		-v "$(CURDIR)/load/k6:/scripts:ro" \
		grafana/k6:2.2.0 run /scripts/backpressure.js
archive-check:
	go test ./internal/incidentarchive

demo-change:
	python3 scripts/demo-change.py

connector-check:
	go run ./cmd/telemetryctl connector validate --file routing/active.json
	go run ./cmd/telemetryctl connector validate --file routing/shadow.json
	go run ./cmd/telemetryctl connector validate --file routing/connectors.example.json
	go test ./internal/connectors ./internal/router

proof-check:
	go test ./internal/proof
	python3 -m py_compile scripts/proof_common.py

proof-verify:
	@test -n "$(FILE)" || (echo "FILE=proof/results/run.tfproof.json is required" >&2; exit 2)
	go run ./cmd/telemetryctl proof verify --file "$(FILE)"

proof-k6-smoke:
	proof/run-k6.sh smoke --execute

proof-k6-sustained:
	proof/run-k6.sh sustained --execute

proof-k6-backpressure:
	proof/run-k6.sh backpressure --execute

proof-kafka-outage:
	proof/kafka-outage.sh --execute

proof-postgres-outage:
	proof/postgres-outage.sh --execute

proof-worker-failover:
	proof/worker-failover.sh --execute

proof-connector-outage:
	proof/connector-outage.sh --execute

proof-backup-restore:
	proof/backup-restore.sh --execute

proof-wal-crash-recovery:
	proof/wal-crash-recovery.sh --execute

proof-edge-replication:
	proof/edge-replication.sh --execute

proof-mesh-failover:
	proof/mesh-failover.sh --execute

proof-cryptographic-lineage:
	proof/cryptographic-lineage.sh --execute

formal-check:
	go test ./internal/formal
	go run ./cmd/formalcheck

proof-formal-verification:
	proof/formal-verification.sh --execute

proof-fastpath-performance:
	proof/fastpath-performance.sh --execute

proof-autonomous-control:
	proof/autonomous-control.sh --execute

autonomy-check:
	go test ./internal/autonomy ./internal/shaping
	go run ./cmd/autonomycheck

plugin-check:
	go test ./internal/wasmplugin
	go run ./cmd/plugincheck --manifest plugins/examples/noop-predictor.manifest.json --module plugins/examples/noop-predictor.wasm

edge-check:
	go test ./internal/fastpath ./internal/lineage ./internal/wal ./internal/replication ./internal/mesh ./internal/edge
	go build ./cmd/edge

intelligence-check:
	go test ./internal/intelligence
	go run ./cmd/telemetryctl intelligence list --help >/dev/null 2>&1 || true
