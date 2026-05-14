# ============================
# Builder stage
# ============================
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

# Build indexer
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags='-w -s -extldflags "-static"' \
    -o indexer ./cmd/indexer/main.go

# Build dispatcher
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags='-w -s -extldflags "-static"' \
    -o dispatcher ./cmd/dispatcher/main.go

# Pull nats-server binary from official image
FROM nats:2.10-alpine AS nats

# ============================
# Runtime stage
# ============================
FROM alpine:3.19

# Redis + supervisor (process manager)
RUN apk add --no-cache redis supervisor ca-certificates

# NATS server from official image
COPY --from=nats /usr/local/bin/nats-server /usr/local/bin/nats-server

WORKDIR /app

COPY --from=builder /app/indexer    /app/indexer
COPY --from=builder /app/dispatcher /app/dispatcher
COPY --from=builder /app/configs    /app/configs

COPY supervisord.conf /etc/supervisord.conf
COPY nats.conf /app/nats.conf

EXPOSE 8080

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
