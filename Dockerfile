# Build stage - pinned to latest stable Go on Alpine
FROM golang:1.25.1-alpine3.21 AS builder
LABEL maintainer="joona@kuori.org"

# Install build dependencies (minimal set)
RUN apk add --no-cache gcc musl-dev git

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies with caching
# This layer will be cached unless go.mod/go.sum changes
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Build with optimizations and caching
# - Use build cache for faster rebuilds
# - Strip debug symbols (-w -s)
# - Static binary for alpine
# - Parallel compilation
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=${TARGETARCH} \
    go build -a \
    -installsuffix cgo \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o acme-dns .

# Runtime stage - pinned to latest stable Alpine for security updates
FROM alpine:3.21.3
LABEL maintainer="joona@kuori.org"
LABEL org.opencontainers.image.source="https://github.com/joohoi/acme-dns"
LABEL org.opencontainers.image.description="Simplified DNS server with a RESTful HTTP API for ACME DNS challenges with Web UI"

# Install runtime dependencies (minimal set, curl for healthcheck)
# Combine into single layer to reduce image size
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    && update-ca-certificates \
    && adduser -D -u 1000 -h /app acmedns \
    && mkdir -p /etc/acme-dns /var/lib/acme-dns \
    && chown -R acmedns:acmedns /etc/acme-dns /var/lib/acme-dns

WORKDIR /app

# Copy binary from builder
COPY --from=builder --chown=acmedns:acmedns /build/acme-dns .

# Copy web UI files (templates and static assets)
COPY --from=builder --chown=acmedns:acmedns /build/web ./web

# Copy example config
COPY --from=builder --chown=acmedns:acmedns /build/config.cfg /etc/acme-dns/config.cfg.example

# Security: Run as non-root user
USER acmedns

# Expose ports
# DNS
EXPOSE 53/tcp 53/udp
# HTTP/HTTPS API
EXPOSE 80/tcp 443/tcp

# Define volumes for persistence
VOLUME ["/etc/acme-dns", "/var/lib/acme-dns"]

# Health check (using curl which is already installed)
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f --max-time 2 http://localhost:80/health || exit 1

# Run the application
ENTRYPOINT ["./acme-dns"]
CMD ["-c", "/etc/acme-dns/config.cfg"]
