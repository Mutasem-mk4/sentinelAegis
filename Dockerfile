# ── Stage 1: Build ──────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk --no-cache add ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o /app/sentinelaegis .

# ── Stage 2: Run ────────────────────────────────────────
FROM alpine:3.19

# Security: run as non-root user
RUN apk --no-cache add ca-certificates \
    && addgroup -S aegis \
    && adduser -S aegis -G aegis

WORKDIR /app

COPY --from=builder /app/sentinelaegis .
COPY frontend/ ./frontend/
COPY assets/ ./assets/

# Drop to non-root
USER aegis

ENV PORT=8080

EXPOSE 8080

# Cloud Run health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

CMD ["./sentinelaegis"]
