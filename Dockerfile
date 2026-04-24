# ── Stage 1: Build ──
FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod ./
# No go.sum needed — zero external dependencies

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/sentinelaegis .

# ── Stage 2: Run ──
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/sentinelaegis .
COPY frontend/ ./frontend/

ENV PORT=8080

EXPOSE 8080

CMD ["./sentinelaegis"]
