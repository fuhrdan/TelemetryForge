FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetryforge-gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/telemetryforge-gateway /telemetryforge-gateway
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/telemetryforge-gateway"]
