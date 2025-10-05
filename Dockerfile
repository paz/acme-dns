# Build stage
FROM golang:1.25-alpine AS builder
LABEL maintainer="joona@kuori.org"

# Install build dependencies
RUN apk add --no-cache gcc musl-dev git ca-certificates

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o acme-dns .

# Runtime stage
FROM alpine:latest
LABEL maintainer="joona@kuori.org"
LABEL org.opencontainers.image.source="https://github.com/joohoi/acme-dns"
LABEL org.opencontainers.image.description="Simplified DNS server with a RESTful HTTP API for ACME DNS challenges with Web UI"

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates && \
    adduser -D -u 1000 acmedns

# Create required directories
RUN mkdir -p /etc/acme-dns /var/lib/acme-dns && \
    chown -R acmedns:acmedns /etc/acme-dns /var/lib/acme-dns

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/acme-dns .

# Copy web UI files (templates and static assets)
COPY --from=builder /build/web ./web

# Copy example config
COPY --from=builder /build/config.cfg /etc/acme-dns/config.cfg.example

# Switch to non-root user
USER acmedns

# Expose ports
# DNS
EXPOSE 53/tcp 53/udp
# HTTP/HTTPS API
EXPOSE 80/tcp 443/tcp

# Define volumes for persistence
VOLUME ["/etc/acme-dns", "/var/lib/acme-dns"]

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:80/health || exit 1

# Run the application
ENTRYPOINT ["./acme-dns"]
CMD ["-c", "/etc/acme-dns/config.cfg"]
