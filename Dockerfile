FROM golang:1.27.1-alpine3.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-gateway ./cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryctl ./cmd/telemetryctl

FROM alpine:3.24.1
RUN addgroup -S telemetryforge && adduser -S -G telemetryforge telemetryforge
COPY --from=build /out/telemetryforge-gateway /usr/local/bin/telemetryforge-gateway
COPY --from=build /out/telemetryforge-worker /usr/local/bin/telemetryforge-worker
COPY --from=build /out/telemetryctl /usr/local/bin/telemetryctl
COPY policies /etc/telemetryforge/policies
USER telemetryforge
EXPOSE 8080 8081
ENTRYPOINT ["/usr/local/bin/telemetryforge-gateway"]
