# Rebuild Instructions for Embedded Web UI

**CRITICAL UPDATE**: The web UI files are now embedded in the binary! You need to rebuild with the latest code.

---

## What Changed

Previously, the web UI required separate `web/templates/` and `web/static/` directories at runtime. This didn't match acme-dns's single-binary design.

**Now**: All templates and static files are embedded directly in the binary using Go's `embed` package. The binary is completely self-contained.

---

## Quick Rebuild (On Your Linux Server)

### Option 1: Build on Server

```bash
cd /opt/acme-dns

# Pull latest code
git pull origin master

# Build the new binary
go build -v -o acme-dns

# Stop current service
systemctl stop acme-dns

# Replace binary (backup old one first)
cp /usr/local/bin/acme-dns /usr/local/bin/acme-dns.backup
cp acme-dns /usr/local/bin/acme-dns

# Start service
systemctl start acme-dns

# Check logs
journalctl -u acme-dns -f
```

### Option 2: Cross-Compile from Windows

On your Windows development machine:

```bash
cd C:\Users\adm.ParisF\acmedns\acme-dns

# Pull latest if needed
git pull

# Build for Linux
$env:GOOS="linux"
$env:GOARCH="amd64"
& "C:\Program Files\Go\bin\go.exe" build -v -o acme-dns

# Copy to server (adjust path as needed)
scp acme-dns root@acme-dns:/tmp/acme-dns

# On server:
# systemctl stop acme-dns
# mv /usr/local/bin/acme-dns /usr/local/bin/acme-dns.backup
# mv /tmp/acme-dns /usr/local/bin/acme-dns
# chmod +x /usr/local/bin/acme-dns
# systemctl start acme-dns
```

---

## Verify Web UI is Working

After rebuild and restart:

```bash
# Check logs for successful initialization
journalctl -u acme-dns | grep "Web UI enabled"

# Should see:
# INFO[0000] Web UI enabled - initializing web components

# Should NOT see any template errors
```

### Test Access

```bash
# Test health endpoint
curl https://auth.busictgroup.com.au/health

# Test login page (should return HTML)
curl https://auth.busictgroup.com.au/login
```

Or open in browser:
```
https://auth.busictgroup.com.au/login
```

---

## Create Admin User

Once the service is running with embedded templates:

```bash
acme-dns --create-admin paris.fuja@empowerict.com.au
```

Enter a secure password (minimum 12 characters).

---

## What's Embedded

The binary now includes:

**Templates** (from `web/templates/`):
- layout.html - Base layout with Bootstrap 5
- login.html - Login page
- dashboard.html - User dashboard
- admin.html - Admin panel

**Static Files** (from `web/static/`):
- css/style.css - Custom styling (3KB)
- js/app.js - JavaScript functionality (10KB)
- img/.gitkeep - Placeholder for future images

**Total embedded size**: ~15KB (minimal impact on binary size)

---

## Troubleshooting

### Issue: Still seeing "pattern matches no files" error

**Cause**: Running old binary

**Solution**:
1. Verify you rebuilt: `acme-dns --version` (should show build date)
2. Check binary location: `which acme-dns`
3. Ensure you replaced the correct binary

### Issue: "404 Not Found" on /static/css/style.css

**Cause**: Static handler not registered correctly

**Solution**:
1. Check you're running the latest code (commit e4bb433 or later)
2. Rebuild with `go build -v`
3. Restart service

### Issue: Templates don't display correctly

**Cause**: Browser cache

**Solution**:
1. Hard refresh: Ctrl+F5 (Windows) or Cmd+Shift+R (Mac)
2. Clear browser cache
3. Try incognito/private window

---

## Verification Checklist

After rebuild:

- [ ] Service starts without errors
- [ ] No "pattern matches no files" error in logs
- [ ] `/login` returns HTML (not 404)
- [ ] `/static/css/style.css` returns CSS
- [ ] `/static/js/app.js` returns JavaScript
- [ ] Admin user creation works
- [ ] Can login to web UI
- [ ] Dashboard displays correctly

---

## Docker Rebuild (If Using Docker)

The Docker image is automatically built by GitHub Actions with embedded files.

Pull the latest image:

```bash
docker pull ghcr.io/paz/acme-dns:latest

# Or rebuild locally
docker build -t acme-dns:local .
```

---

## Development Notes

### Building Locally

```bash
# Standard build
go build -v

# With race detector (testing only)
go build -v -race

# Optimized for production
go build -v -ldflags="-w -s" -trimpath
```

### Verifying Embedded Files

Check what's embedded in the binary:

```bash
# List embedded files (requires go 1.16+)
go list -f '{{.EmbedFiles}}' ./web

# Should show:
# [templates/layout.html templates/login.html templates/dashboard.html templates/admin.html static/css/style.css static/js/app.js static/img/.gitkeep]
```

---

## What You Don't Need Anymore

With embedded files, these are **NOT** required at runtime:

- ❌ web/templates/ directory
- ❌ web/static/ directory
- ❌ Deploying web folder structure
- ❌ Configuring template paths

**Still Required**:
- ✅ acme-dns binary (with embedded files)
- ✅ /etc/acme-dns/config.cfg

---

## Summary

1. **Pull latest code** from GitHub
2. **Rebuild** the binary (`go build`)
3. **Replace** the old binary
4. **Restart** the service
5. **Create** admin user
6. **Access** web UI at `/login`

The web UI now works out of the box with zero external file dependencies!

---

**Updated**: 2025-10-05
**Commit**: e4bb433
**Build tested**: ✅ Windows & Linux
