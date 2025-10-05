# GitHub Actions Workflow Status

## 🔄 Current Build: IN PROGRESS

**Run ID**: 18255184316
**Commit**: `3daec0b` - Fix Docker workflow attestation step
**Started**: 2025-10-05 06:38:18 UTC
**Status**: Building (multi-platform)

### Monitor Commands

```bash
# Check status
gh run list --workflow=docker-publish.yml --limit 1

# View run details
gh run view 18255184316

# Watch logs (when available)
gh run watch 18255184316

# Or use helper script
.\gh-helper.ps1  # Choose option 3
```

---

## 🐛 Issue Fixed

### Problem
Previous builds were failing at the attestation step:
```
Error: One of subject-path or subject-digest must be provided
```

### Root Cause
- Line 76 used `${{ steps.meta.outputs.digest }}`
- But `docker/metadata-action` doesn't output digest
- The `docker/build-push-action` outputs digest, but had no ID

### Solution Applied
```yaml
# Added ID to build step
- name: Build and push Docker image
  id: build  # <-- ADDED THIS
  uses: docker/build-push-action@v5
  # ... rest of config

# Fixed attestation step
- name: Generate artifact attestation
  uses: actions/attest-build-provenance@v1
  with:
    subject-name: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
    subject-digest: ${{ steps.build.outputs.digest }}  # <-- FIXED THIS
```

---

## 📊 Previous Build History

| Run | Status | Commit | Issue |
|-----|--------|--------|-------|
| 18255184316 | ⏳ In Progress | 3daec0b | (current) |
| 18255029832 | ❌ Failed | 11fdf24 | Attestation: missing digest |
| 18255004776 | ❌ Failed | c6ed6b2 | Attestation: missing digest |

---

## ✅ What Will Happen When Build Succeeds

### 1. Images Published to GHCR
```
ghcr.io/paz/acme-dns:latest
ghcr.io/paz/acme-dns:master
ghcr.io/paz/acme-dns:master-3daec0b
```

### 2. Multi-Platform Support
- `linux/amd64` - Standard x86_64 servers
- `linux/arm64` - ARM-based servers (Raspberry Pi, ARM VPS)

### 3. Attestation Created
- Build provenance attestation
- Signed with GitHub's OIDC tokens
- Verifiable with `gh attestation verify`

---

## 🚀 Next Steps After Build Completes

### 1. Verify Build Success
```bash
gh run list --limit 1
# Should show: completed	success
```

### 2. Make Package Public
1. Go to: https://github.com/paz?tab=packages
2. Click on `acme-dns` package
3. **Package settings** → **Change visibility** → **Public**

### 3. Test Pull Image
```bash
docker pull ghcr.io/paz/acme-dns:latest
```

### 4. Verify Image
```bash
# Check image details
docker inspect ghcr.io/paz/acme-dns:latest

# Verify it includes web UI files
docker run --rm ghcr.io/paz/acme-dns:latest ls -la /app/web/

# Should show:
# drwxr-xr-x web/templates/
# drwxr-xr-x web/static/
```

### 5. Deploy to Portainer
Use one of these methods:

**Method 1: Direct Pull** (Easiest)
```yaml
services:
  acmedns:
    image: ghcr.io/paz/acme-dns:latest
    # ... rest of config from docker-compose.yml
```

**Method 2: Stack from Repository**
- Stack URL: `https://github.com/paz/acme-dns`
- Compose path: `docker-compose.yml`
- Auto-pull on update

**Method 3: Local Build**
- Clone repo on Linux server
- Build: `docker build -t acme-dns:local .`
- Deploy locally built image

---

## 🔍 Troubleshooting

### Build Still In Progress After 20 Minutes
```bash
# Check detailed logs
gh run view 18255184316 --log

# Look for errors in build steps
gh run view 18255184316 --job=<job-id> --log
```

### Build Failed Again
```bash
# Get failure details
gh run view 18255184316 --log | tail -100

# Check specific step
gh run view 18255184316 --log | grep -A 20 "error"
```

### Re-run Failed Build
```bash
gh run rerun 18255184316
```

### Cancel Running Build
```bash
gh run cancel 18255184316
```

---

## 📈 Build Timeline (Estimated)

```
0:00  - Checkout repository          ✓
0:30  - Set up Docker Buildx         ✓
1:00  - Login to GHCR                ✓
1:30  - Extract metadata             ✓
2:00  - Build linux/amd64            ⏳ (5-7 min)
8:00  - Build linux/arm64            ⏳ (5-7 min)
14:00 - Push to registry             ⏳ (1-2 min)
15:00 - Generate attestation         ⏳ (30 sec)
15:30 - Complete                     🎉
```

---

## 🎯 Success Criteria

- [⏳] Multi-platform build completes
- [⏳] Images pushed to GHCR
- [⏳] Attestation step succeeds (was failing before)
- [⏳] Tags created: latest, master, master-<sha>
- [ ] Package made public (manual step)
- [ ] Image tested with `docker pull`
- [ ] Deployed to Portainer

---

## 📞 Quick Reference

### URLs
- **Workflow Run**: https://github.com/paz/acme-dns/actions/runs/18255184316
- **All Actions**: https://github.com/paz/acme-dns/actions
- **Packages**: https://github.com/paz?tab=packages
- **GHCR**: ghcr.io/paz/acme-dns

### Commands
```bash
# Status check
gh run list --limit 1

# Watch build
gh run watch

# View logs
gh run view --log

# After success, pull image
docker pull ghcr.io/paz/acme-dns:latest

# Deploy
cd /path/to/acme-dns
docker-compose up -d
```

---

**Last Updated**: 2025-10-05 14:40 UTC
**Current Status**: 🟡 Building (expected ~12 more minutes)

---

*Monitoring tip: Run `gh run watch` in a terminal to see live updates, or use `.\gh-helper.ps1` option 3 for an interactive view.*
