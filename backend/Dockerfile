# Force fresh build to pick up indexer changes
# ── Build Stage ───────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# Install git for private module cloning
RUN apk add --no-cache git ca-certificates

# Set the working directory to /app (where b2b-merchant will live)
WORKDIR /app

# We need the indexer as a sibling for the 'replace' directive in go.mod.
# Path: /indexer
# Path: /app (b2b-merchant)

# Accept GitHub Token as build argument
ARG GITHUB_TOKEN

# Force a fresh clone by changing this timestamp: 2026-04-24T14:40:00
RUN git clone https://${GITHUB_TOKEN}@github.com/tantasui/indexer-linq-b2b.git /indexer

# Copy b2b-merchant source code
COPY . .

# Build the application
# We build from the cmd directory
RUN go build -o b2b-server ./cmd/main.go

# ── Runtime Stage ─────────────────────────────────────────────────────────────
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/b2b-server .

# Copy static assets
# Ensure the web/ directory is in the same relative path as expected by the code
COPY --from=builder /app/web ./web

# Expose the application port
EXPOSE 8081

# Command to run the application
CMD ["./b2b-server"]
