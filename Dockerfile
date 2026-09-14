FROM golang:1.27.1-alpine3.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-gateway ./cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-edge ./cmd/edge
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-router ./cmd/router
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryctl ./cmd/telemetryctl
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-autonomycheck ./cmd/autonomycheck
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-plugincheck ./cmd/plugincheck
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-ebpfcheck ./cmd/ebpfcheck
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-multicloudcheck ./cmd/multicloudcheck

FROM alpine:3.24.1
RUN addgroup -S telemetryforge && adduser -S -G telemetryforge telemetryforge
COPY --from=build /out/telemetryforge-gateway /usr/local/bin/telemetryforge-gateway
COPY --from=build /out/telemetryforge-edge /usr/local/bin/telemetryforge-edge
COPY --from=build /out/telemetryforge-worker /usr/local/bin/telemetryforge-worker
COPY --from=build /out/telemetryforge-router /usr/local/bin/telemetryforge-router
COPY --from=build /out/telemetryctl /usr/local/bin/telemetryctl
COPY --from=build /out/telemetryforge-autonomycheck /usr/local/bin/telemetryforge-autonomycheck
COPY --from=build /out/telemetryforge-plugincheck /usr/local/bin/telemetryforge-plugincheck
COPY --from=build /out/telemetryforge-ebpfcheck /usr/local/bin/telemetryforge-ebpfcheck
COPY --from=build /out/telemetryforge-multicloudcheck /usr/local/bin/telemetryforge-multicloudcheck
COPY policies /etc/telemetryforge/policies
COPY routing /etc/telemetryforge/routing
COPY shaping /etc/telemetryforge/shaping
USER telemetryforge
EXPOSE 8080 8081 8082 8083
ENTRYPOINT ["/usr/local/bin/telemetryforge-gateway"]
