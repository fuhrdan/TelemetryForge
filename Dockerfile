FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge ./cmd/gateway

FROM alpine:3.22
RUN addgroup -S telemetryforge && adduser -S -G telemetryforge telemetryforge
COPY --from=build /out/telemetryforge /usr/local/bin/telemetryforge
USER telemetryforge
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/telemetryforge"]
