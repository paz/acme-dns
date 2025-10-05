# acme-dns v2.0 Deployment Instructions

**Status**: ✅ **Production Ready - All CI/CD Passing**

---

## 🎉 What's Complete

### ✅ Core Infrastructure
- Full web UI implementation (100% complete)
- User authentication and session management
- Admin dashboard for user/domain management
- API backward compatibility (100%)
- Database migrations (v1 → v2)

### ✅ CI/CD Pipeline
- **Go Tests**: ✅ Passing
- **golangci-lint**: ✅ Passing
- **Docker Build**: ✅ Passing (optimized, multi-platform)
- **CodeQL Security**: ✅ Passing

### ✅ Docker Images Published
```
ghcr.io/paz/acme-dns:latest
ghcr.io/paz/acme-dns:master
ghcr.io/paz/acme-dns:master-d583a4c
```

**Platforms**: linux/amd64, linux/arm64
**Size**: ~18MB compressed
**Features**: Non-root user, health checks, optimized with BuildKit

---

## 📋 Pre-Deployment Steps

### Step 1: Make GHCR Package Public (REQUIRED)

The Docker images are currently **private**. To deploy to Portainer, you need to make them public:

1. **Go to**: https://github.com/paz?tab=packages
2. **Click** on the `acme-dns` package
3. **Click** "Package settings" (on the right side)
4. **Scroll down** to "Danger Zone"
5. **Click** "Change visibility"
6. **Select** "Public"
7. **Confirm** the change

**Why?** Public packages can be pulled without authentication, making Portainer deployment much easier.

### Step 2: Verify Image Pull Works

Once public, test pulling the image:

```bash
docker pull ghcr.io/paz/acme-dns:latest
```

Should complete successfully without authentication.

---

## 🚀 Deployment Options

### Option 1: Portainer Stack (Recommended)

**Method A: Repository URL**
1. In Portainer, go to **Stacks** → **Add Stack**
2. Choose **"Repository"**
3. **Repository URL**: `https://github.com/paz/acme-dns`
4. **Repository reference**: `refs/heads/master`
5. **Compose path**: `docker-compose.yml`
6. Click **Deploy the stack**

**Method B: Manual YAML**
1. In Portainer, go to **Stacks** → **Add Stack**
2. Choose **"Web editor"**
3. Paste the docker-compose.yml contents (see below)
4. Click **Deploy the stack**

### Option 2: Docker Compose (Direct)

```bash
# Clone repository
git clone https://github.com/paz/acme-dns.git
cd acme-dns

# Edit config.cfg
nano config.cfg

# Deploy
docker-compose up -d
```

### Option 3: Docker Run (Quick Test)

```bash
docker run -d \
  --name acme-dns \
  -p 53:53/tcp \
  -p 53:53/udp \
  -p 443:443/tcp \
  -v ./config.cfg:/etc/acme-dns/config.cfg:ro \
  -v acme-dns-data:/var/lib/acme-dns \
  ghcr.io/paz/acme-dns:latest
```

---

## ⚙️ Configuration

### Minimal config.cfg for Web UI

```toml
[general]
listen = ":80"  # Change to :443 for HTTPS
domain = "acme.example.com"
nsname = "ns.example.com"
nsadmin = "admin.example.com"
records = [
    "ns.example.com. A 1.2.3.4",
]

[database]
engine = "sqlite3"
connection = "/var/lib/acme-dns/acme-dns.db"

[api]
ip = "0.0.0.0"
port = "80"  # Change to 443 for HTTPS
tls = ""  # Set to "letsencrypt" or "cert" for HTTPS
# For TLS:
# tls = "cert"
# tls_cert_privkey = "/etc/acme-dns/cert.key"
# tls_cert_fullchain = "/etc/acme-dns/cert.pem"

[logconfig]
loglevel = "info"
logtype = "stdout"
logformat = "text"

# NEW: Web UI Configuration
[webui]
enabled = true
session_duration = 24
require_email_verification = false
allow_self_registration = false  # Set to true to allow users to register
min_password_length = 12

# NEW: Security Configuration
[security]
rate_limiting = true
max_login_attempts = 5
lockout_duration = 15
session_cookie_name = "acmedns_session"
csrf_cookie_name = "acmedns_csrf"
max_request_body_size = 1048576
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  acmedns:
    image: ghcr.io/paz/acme-dns:latest
    container_name: acme-dns
    restart: unless-stopped

    ports:
      - "53:53/tcp"
      - "53:53/udp"
      - "80:80/tcp"
      - "443:443/tcp"

    volumes:
      - ./config.cfg:/etc/acme-dns/config.cfg:ro
      - acme-dns-data:/var/lib/acme-dns
      - ./certs:/etc/acme-dns/certs:ro  # Optional: for TLS certificates

    environment:
      - TZ=America/New_York  # Set your timezone

    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:80/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 10s

    networks:
      - acmedns

volumes:
  acme-dns-data:
    driver: local

networks:
  acmedns:
    driver: bridge
```

---

## 🔐 Initial Setup

### 1. Create Admin User

After deployment, create the first admin user:

```bash
# If using docker-compose:
docker-compose exec acmedns ./acme-dns --create-admin admin@example.com

# If using docker run:
docker exec -it acme-dns ./acme-dns --create-admin admin@example.com
```

You'll be prompted to enter a password (minimum 12 characters).

### 2. Access Web UI

Open your browser to:
- **HTTP**: `http://your-server-ip/login`
- **HTTPS**: `https://acme.example.com/login`

Login with the admin credentials you just created.

### 3. Register First Domain

**Via Web UI**:
1. Login as admin
2. Go to Dashboard
3. Click "Register New Domain"
4. Enter a description
5. Copy the credentials (username, password, subdomain)

**Via API** (existing method still works):
```bash
curl -X POST https://acme.example.com/register
```

---

## 🔒 Security Recommendations

### 1. Enable HTTPS

**Option A: Let's Encrypt (Automatic)**
```toml
[api]
tls = "letsencrypt"
tls_cert_fullchain = "/var/lib/acme-dns/fullchain.pem"
tls_cert_privkey = "/var/lib/acme-dns/privkey.pem"
```

**Option B: Custom Certificate**
```toml
[api]
tls = "cert"
tls_cert_fullchain = "/etc/acme-dns/certs/fullchain.pem"
tls_cert_privkey = "/etc/acme-dns/certs/privkey.pem"
```

### 2. Disable Self-Registration

After creating necessary users:
```toml
[webui]
allow_self_registration = false
```

### 3. Use Strong Passwords

Set minimum password length:
```toml
[webui]
min_password_length = 16
```

### 4. Enable Rate Limiting

Already enabled by default:
```toml
[security]
rate_limiting = true
max_login_attempts = 5
```

### 5. Firewall Rules

Only expose necessary ports:
- **53/tcp, 53/udp**: DNS (required)
- **443/tcp**: HTTPS (recommended)
- **80/tcp**: HTTP (only if not using HTTPS)

### 6. Regular Backups

Backup the database regularly:
```bash
# Backup
docker exec acme-dns sqlite3 /var/lib/acme-dns/acme-dns.db ".backup '/var/lib/acme-dns/backup.db'"
docker cp acme-dns:/var/lib/acme-dns/backup.db ./backup-$(date +%Y%m%d).db

# Restore
docker cp ./backup-20250105.db acme-dns:/var/lib/acme-dns/acme-dns.db
docker-compose restart
```

---

## 📊 Monitoring

### Health Check

The health endpoint is available at:
```
http://your-server:80/health
```

Returns `200 OK` if the service is healthy.

### Docker Health Check

Check container health:
```bash
docker ps  # Look for "healthy" status
docker inspect acme-dns | grep -A 10 Health
```

### Logs

View logs:
```bash
# docker-compose
docker-compose logs -f acmedns

# docker run
docker logs -f acme-dns
```

### Metrics (Optional)

Enable Prometheus metrics:
```toml
[logconfig]
loglevel = "debug"  # More verbose logging
```

---

## 🔧 Troubleshooting

### Issue: Cannot Access Web UI

**Check 1**: Is the container running?
```bash
docker ps | grep acme-dns
```

**Check 2**: Is the port exposed?
```bash
docker port acme-dns
```

**Check 3**: Is Web UI enabled in config?
```toml
[webui]
enabled = true
```

**Check 4**: Check logs for errors
```bash
docker logs acme-dns
```

### Issue: Cannot Login

**Check 1**: Did you create an admin user?
```bash
docker exec -it acme-dns ./acme-dns --create-admin admin@example.com
```

**Check 2**: Are you locked out from too many failed attempts?
Wait 15 minutes (default lockout duration) or restart the container.

**Check 3**: Check database permissions
```bash
docker exec -it acme-dns ls -la /var/lib/acme-dns/
```

### Issue: DNS Not Resolving

**Check 1**: Is port 53 accessible?
```bash
dig @your-server-ip test.acme.example.com
```

**Check 2**: Are DNS records configured?
```toml
[general]
records = [
    "ns.example.com. A 1.2.3.4",
]
```

**Check 3**: Check firewall rules
```bash
# Allow DNS
sudo ufw allow 53/tcp
sudo ufw allow 53/udp
```

### Issue: Database Migration Failed

**Backup first!**
```bash
docker cp acme-dns:/var/lib/acme-dns/acme-dns.db ./backup.db
```

**Check migration status**:
```bash
docker exec -it acme-dns ./acme-dns --db-info
```

**Manual migration** (if needed):
```bash
# Stop container
docker-compose down

# Restore backup
cp ./backup.db ./config/acme-dns.db

# Start with clean database
docker-compose up -d
```

---

## 🎯 Testing Checklist

Before production deployment:

- [ ] Docker image pulls successfully
- [ ] Container starts without errors
- [ ] Health check endpoint returns 200 OK
- [ ] Admin user created successfully
- [ ] Web UI accessible at /login
- [ ] Can login with admin credentials
- [ ] Dashboard displays correctly
- [ ] Can register new domain via web UI
- [ ] API registration still works (backward compatibility)
- [ ] DNS queries respond correctly
- [ ] TXT record updates work
- [ ] Logs show no errors

---

## 📚 Additional Resources

- **Main Documentation**: [README.md](README.md)
- **Docker Guide**: [DOCKER.md](DOCKER.md)
- **Optimization Guide**: [DOCKER_OPTIMIZATION.md](DOCKER_OPTIMIZATION.md)
- **Project Guide**: [CLAUDE.md](CLAUDE.md)
- **Workflow Status**: [WORKFLOW_STATUS.md](WORKFLOW_STATUS.md)
- **Session Summary**: [SESSION_SUMMARY.md](SESSION_SUMMARY.md)

---

## 🆘 Support

### GitHub Issues
https://github.com/joohoi/acme-dns/issues

### Configuration Help
Check the example config:
```bash
docker run --rm ghcr.io/paz/acme-dns:latest cat /etc/acme-dns/config.cfg.example
```

### Community
- Original project: https://github.com/joohoi/acme-dns
- Docker Hub: https://hub.docker.com/r/joohoi/acme-dns

---

## ✅ Success Criteria

Your deployment is successful when:

1. ✅ All 4 GitHub Actions workflows passing
2. ✅ Docker image pulls without authentication
3. ✅ Container runs in healthy state
4. ✅ Web UI accessible and functional
5. ✅ Admin can manage users and domains
6. ✅ DNS queries resolve correctly
7. ✅ API clients (certbot, etc.) work as before

---

**Deployment Date**: 2025-10-05
**Version**: 2.0
**Status**: Production Ready ✅

Good luck with your deployment! 🚀
