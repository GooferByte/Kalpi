# ── Stage 1: Build ────────────────────────────────────────────────────────────
# Use the official Go image with Alpine for a small build environment.
FROM golang:1.22-alpine AS builder

# Install git (needed for modules that use VCS)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Cache dependency download as a separate layer
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /kalpi ./cmd/server

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
# Alpine: tiny image (~10MB) that includes wget for Docker healthchecks.
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata wget

COPY --from=builder /kalpi /kalpi

EXPOSE 8080

ENTRYPOINT ["/kalpi"]
