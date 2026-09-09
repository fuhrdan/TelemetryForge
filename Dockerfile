FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-gateway ./cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-worker ./cmd/worker

FROM alpine:3.22
RUN addgroup -S telemetryforge && adduser -S -G telemetryforge telemetryforge
COPY --from=build /out/telemetryforge-gateway /usr/local/bin/telemetryforge-gateway
COPY --from=build /out/telemetryforge-worker /usr/local/bin/telemetryforge-worker
USER telemetryforge
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/telemetryforge-gateway"]
