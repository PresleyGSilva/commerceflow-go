# syntax=docker/dockerfile:1
FROM golang:1.23-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /out/api /app/api
COPY migrations /app/migrations
COPY openapi.yaml /app/openapi.yaml
EXPOSE 8082
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8082/healthz || exit 1
ENTRYPOINT ["/app/api"]
