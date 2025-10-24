FROM golang:1.25.3-alpine AS builder

# Install build dependencies INCLUDING libheif for AVIF support
RUN apk add --no-cache \
    vips-dev \
    libheif-dev \
    libjpeg-turbo-dev \
    libpng-dev \
    libwebp-dev \
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

# Build - optimized for production
RUN CGO_ENABLED=1 GOOS=linux \
    go build -ldflags="-s -w" -o server ./cmd/server

# Final stage
FROM alpine:latest

# Install runtime dependencies INCLUDING libheif for AVIF
RUN apk add --no-cache \
    vips \
    libheif \
    libjpeg-turbo \
    libpng \
    libwebp \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

EXPOSE 3000

CMD ["./server"]