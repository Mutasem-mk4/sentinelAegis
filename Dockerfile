# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o sentinelaegis .

# Final Stage
FROM alpine:3.19

# Non-root user for security
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary and frontend assets
COPY --from=builder /app/sentinelaegis .
COPY --from=builder /app/frontend ./frontend

# Change ownership
RUN chown -R appuser:appuser /app
USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./sentinelaegis"]
