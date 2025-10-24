FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    vips-dev \
    gcc \
    g++ \
    make \
    pkgconfig

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy entire source
COPY . .

# Verify files exist (debug)
RUN ls -la && ls -la cmd/ && ls -la cmd/server/

# Build - simplified path
RUN CGO_ENABLED=1 GOOS=linux \
    go build -o server ./cmd/server

# Final stage
FROM alpine:latest

RUN apk add --no-cache \
    vips \
    ca-certificates \
    tzdata

RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

WORKDIR /app

COPY --from=builder /app/server .

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 3000

CMD ["./server"]