# ==========================================
# Build Stage
# ==========================================
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install ca-certificates and git
RUN apk add --no-cache ca-certificates git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/bot ./cmd/bot

# ==========================================
# Production Runtime Stage
# ==========================================
FROM alpine:3.21

WORKDIR /app

# Install ca-certificates and tzdata for timezones & HTTPS
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy compiled binary and seed knowledge base from builder
COPY --from=builder /app/bin/bot /app/bot
COPY --from=builder /app/data /app/data

USER appuser

ENV PORT=8080
EXPOSE 8080

CMD ["/app/bot"]
