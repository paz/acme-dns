# Docker Deployment Guide for acme-dns v2.0

## Quick Start

### Using Pre-built Image from GHCR

```bash
# Pull the latest image
docker pull ghcr.io/joohoi/acme-dns:latest

# Run with docker run
docker run -d \
  --name acme-dns \
  -p 53:53/tcp \
  -p 53:53/udp \
  -p 80:80/tcp \
  -p 443:443/tcp \
  -v $(pwd)/config:/etc/acme-dns:ro \
  -v acme-dns-data:/var/lib/acme-dns \
  ghcr.io/joohoi/acme-dns:latest
```

### Using Docker Compose

```bash
# Create config directory
mkdir -p config

# Copy example config
docker run --rm ghcr.io/joohoi/acme-dns:latest \
  cat /etc/acme-dns/config.cfg.example > config/config.cfg

# Edit config.cfg as needed
nano config/config.cfg

# Start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Create admin user
docker-compose exec acmedns ./acme-dns --create-admin admin@example.com
```

## Building from Source

### Local Build

```bash
# Build the image
docker build -t acme-dns:local .

# Run it
docker run -d \
  --name acme-dns \
  -p 53:53/tcp -p 53:53/udp \
  -p 80:80/tcp -p 443:443/tcp \
  -v $(pwd)/config:/etc/acme-dns:ro \
  -v acme-dns-data:/var/lib/acme-dns \
  acme-dns:local
```

### Multi-platform Build

```bash
# Enable BuildKit
export DOCKER_BUILDKIT=1

# Build for multiple platforms
docker buildx create --use
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/joohoi/acme-dns:latest \
  --push \
  .
```

## Configuration

### Required Configuration Changes

1. **Database Settings** (`config/config.cfg`):
```toml
[database]
engine = "sqlite3"
connection = "/var/lib/acme-dns/acme-dns.db"
```

2. **Enable Web UI**:
```toml
[webui]
enabled = true
session_duration = 24
allow_self_registration = false
min_password_length = 12
```

3. **Network Settings**:
```toml
[general]
listen = "0.0.0.0"
domain = "acme.yourdomain.com"

[api]
ip = "0.0.0.0"
port = "80"  # or 443 for TLS
```

## Portainer Deployment

### Stack Deploy (Recommended)

1. **In Portainer**, go to **Stacks** → **Add Stack**
2. **Name**: `acme-dns`
3. **Build method**: `Repository`
4. **Repository URL**: `https://github.com/joohoi/acme-dns`
5. **Compose file path**: `docker-compose.yml`
6. **Environment variables** (optional):
   - `CONFIG_PATH=/path/to/config.cfg`
7. **Deploy the stack**

### Using Web Editor

1. **Copy docker-compose.yml** to Portainer stack editor
2. **Adjust volumes** to match your Portainer volume structure
3. **Add environment variables** if needed
4. **Deploy**

### Using GHCR Image in Portainer

```yaml
version: '3.8'
services:
  acmedns:
    image: ghcr.io/joohoi/acme-dns:latest
    container_name: acme-dns
    restart: unless-stopped
    ports:
      - "53:53/tcp"
      - "53:53/udp"
      - "80:80/tcp"
    volumes:
      - /path/to/your/config.cfg:/etc/acme-dns/config.cfg:ro
      - acme-dns-data:/var/lib/acme-dns
    networks:
      - proxy  # or your network name
volumes:
  acme-dns-data:
networks:
  proxy:
    external: true
```

## Post-Deployment Steps

### 1. Verify Container is Running

```bash
# Docker
docker ps | grep acme-dns

# Docker Compose
docker-compose ps

# Portainer: Check container status in UI
```

### 2. Check Logs

```bash
# Docker
docker logs acme-dns

# Docker Compose
docker-compose logs -f acmedns

# Portainer: View logs in container details
```

### 3. Test Health Check

```bash
# Test from host
curl http://localhost:80/health

# Test from inside container
docker exec acme-dns wget -qO- http://localhost:80/health
```

### 4. Create First Admin User

```bash
# Docker
docker exec -it acme-dns ./acme-dns --create-admin admin@example.com

# Docker Compose
docker-compose exec acmedns ./acme-dns --create-admin admin@example.com

# Portainer: Use Console → Connect to container
./acme-dns --create-admin admin@example.com
```

### 5. Access Web UI

Navigate to: `http://your-server-ip/login` or `https://acme.yourdomain.com/login`

## Persistence

### Volumes

The container uses two volumes:

1. **Configuration** (`/etc/acme-dns`):
   - Mounted as read-only
   - Contains `config.cfg`
   - Edit on host, restart container to apply

2. **Data** (`/var/lib/acme-dns`):
   - Database file
   - Persistent across restarts
   - **BACKUP THIS REGULARLY**

### Backup Strategy

```bash
# Backup database
docker run --rm \
  -v acme-dns-data:/data \
  -v $(pwd):/backup \
  alpine tar czf /backup/acme-dns-backup-$(date +%Y%m%d).tar.gz -C /data .

# Restore database
docker run --rm \
  -v acme-dns-data:/data \
  -v $(pwd):/backup \
  alpine sh -c "cd /data && tar xzf /backup/acme-dns-backup-YYYYMMDD.tar.gz"
```

## Networking

### Reverse Proxy Setup (Traefik/Nginx/Caddy)

#### Traefik Labels

```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.acmedns.rule=Host(`acme.yourdomain.com`)"
  - "traefik.http.routers.acmedns.entrypoints=websecure"
  - "traefik.http.routers.acmedns.tls.certresolver=letsencrypt"
  - "traefik.http.services.acmedns.loadbalancer.server.port=80"
  # DNS (requires Traefik TCP/UDP support)
  - "traefik.tcp.routers.acmedns-dns.rule=HostSNI(`*`)"
  - "traefik.tcp.routers.acmedns-dns.entrypoints=dns-tcp"
  - "traefik.tcp.services.acmedns-dns.loadbalancer.server.port=53"
  - "traefik.udp.routers.acmedns-dns-udp.entrypoints=dns-udp"
  - "traefik.udp.services.acmedns-dns-udp.loadbalancer.server.port=53"
```

#### Nginx Proxy

```nginx
# /etc/nginx/sites-available/acme-dns
upstream acmedns {
    server 127.0.0.1:8080;  # Adjust port
}

server {
    listen 80;
    server_name acme.yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name acme.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://acmedns;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### DNS Port 53 Considerations

Port 53 requires privileged access:

**Option 1**: Run with `--privileged` (not recommended)
```bash
docker run --privileged ...
```

**Option 2**: Use `--cap-add=NET_BIND_SERVICE`
```bash
docker run --cap-add=NET_BIND_SERVICE ...
```

**Option 3**: Map to high port and use iptables redirect
```bash
# Map to port 5353
docker run -p 5353:53/udp -p 5353:53/tcp ...

# Redirect port 53 to 5353
iptables -t nat -A PREROUTING -p udp --dport 53 -j REDIRECT --to-port 5353
iptables -t nat -A PREROUTING -p tcp --dport 53 -j REDIRECT --to-port 5353
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker logs acme-dns

# Common issues:
# 1. Port already in use
sudo netstat -tulpn | grep :53
sudo netstat -tulpn | grep :80

# 2. Configuration file not found
docker exec acme-dns ls -la /etc/acme-dns/

# 3. Permission issues
docker exec acme-dns ls -la /var/lib/acme-dns/
```

### Database Migration Failed

```bash
# Check database version
docker exec acme-dns ./acme-dns --db-info

# Check logs for migration errors
docker logs acme-dns | grep -i migration

# Manual migration (if needed)
docker exec acme-dns ./acme-dns --migrate
```

### Web UI Not Loading

```bash
# Check if web UI is enabled
docker exec acme-dns cat /etc/acme-dns/config.cfg | grep -A 5 "\[webui\]"

# Check if templates are present
docker exec acme-dns ls -la /app/web/templates/

# Check for errors
docker logs acme-dns | grep -i "web ui"
```

### Cannot Create Admin User

```bash
# Check database connection
docker exec acme-dns ./acme-dns --db-info

# Try with full path
docker exec -it acme-dns /app/acme-dns --create-admin admin@example.com

# Check user was created
docker exec acme-dns sqlite3 /var/lib/acme-dns/acme-dns.db "SELECT * FROM users;"
```

## GitHub Container Registry (GHCR)

### Available Tags

- `latest` - Latest stable release from master branch
- `v2.x.x` - Specific version tags
- `master` - Latest commit on master branch
- `sha-abc123` - Specific commit SHA

### Pull Image

```bash
# Latest version
docker pull ghcr.io/joohoi/acme-dns:latest

# Specific version
docker pull ghcr.io/joohoi/acme-dns:v2.0.0

# Specific commit
docker pull ghcr.io/joohoi/acme-dns:sha-abc123
```

### Image Verification

Images are built with attestation and signed with GitHub's OIDC tokens.

```bash
# View image labels
docker inspect ghcr.io/joohoi/acme-dns:latest | jq '.[0].Config.Labels'

# Verify build provenance (requires GitHub CLI)
gh attestation verify oci://ghcr.io/joohoi/acme-dns:latest
```

## Production Deployment Checklist

- [ ] Configure proper DNS records
- [ ] Set up TLS certificates (Let's Encrypt)
- [ ] Enable Web UI in config
- [ ] Configure database backups
- [ ] Set up monitoring/alerting
- [ ] Create admin user
- [ ] Test API endpoints
- [ ] Test Web UI login
- [ ] Configure firewall rules (ports 53, 80, 443)
- [ ] Set up reverse proxy (if needed)
- [ ] Enable health checks
- [ ] Test failover/restart scenarios
- [ ] Document admin credentials securely

## Resources

- **Main Repository**: https://github.com/joohoi/acme-dns
- **Container Registry**: https://ghcr.io/joohoi/acme-dns
- **Documentation**: See CLAUDE.md, INTEGRATION_COMPLETE.md
- **Issues**: https://github.com/joohoi/acme-dns/issues

## Security Notes

- Container runs as non-root user `acmedns` (UID 1000)
- Config mounted as read-only
- Minimal attack surface (Alpine base)
- Regular security updates via base image
- No unnecessary packages installed
- Healthcheck for monitoring
- Secrets should be passed via Docker secrets or environment variables (not in config files in production)
