# Web UI Testing Guide

## Prerequisites

1. **Enable Web UI in config.cfg:**
   ```toml
   [webui]
   enabled = true
   session_duration = 24
   allow_self_registration = true
   min_password_length = 12
   ```

2. **Create Admin User:**
   ```bash
   ./acme-dns --create-admin admin@example.com
   # Enter password when prompted (min 12 chars)
   ```

3. **Start acme-dns:**
   ```bash
   ./acme-dns -c config.cfg
   ```

## Manual Testing Checklist

### 1. Root Path (/)
- [ ] Navigate to `http://your-server/`
- [ ] Should redirect to `/login`
- [ ] **Expected:** 302 redirect to login page

### 2. Login Page (/login)
- [ ] Navigate to `http://your-server/login`
- [ ] **Expected:** Full HTML page with login form
- [ ] **Not expected:** Empty page or just a newline
- [ ] Page should contain:
  - [ ] Email input field
  - [ ] Password input field
  - [ ] Login button
  - [ ] Bootstrap styling
  - [ ] "acme-dns" branding

### 3. Login with Invalid Credentials
- [ ] Enter wrong email/password
- [ ] Click "Login"
- [ ] **Expected:** Redirect back to /login with error message
- [ ] Error should say "Invalid email or password"

### 4. Login with Valid Credentials
- [ ] Enter correct admin email and password
- [ ] Click "Login"
- [ ] **Expected:** Redirect to `/dashboard`
- [ ] Session cookie should be set
- [ ] No errors in browser console

### 5. Dashboard (/dashboard)
- [ ] Should see dashboard page after login
- [ ] **Expected content:**
  - [ ] "Dashboard" heading
  - [ ] "Register New Domain" button
  - [ ] Table (or "no domains" message if empty)
  - [ ] Navigation bar with email and logout
  - [ ] Admin link (if admin user)

### 6. Dashboard Without Auth
- [ ] Open new incognito/private window
- [ ] Navigate to `http://your-server/dashboard`
- [ ] **Expected:** Redirect to `/login`
- [ ] Should NOT show dashboard content

### 7. Register New Domain
- [ ] Click "Register New Domain" on dashboard
- [ ] **Expected:** Modal dialog appears
- [ ] Fill in description (optional)
- [ ] Fill in allowed IPs (optional)
- [ ] Click "Register Domain"
- [ ] **Expected:**
  - [ ] Modal closes
  - [ ] Page reloads
  - [ ] New domain appears in table

### 8. View Domain Credentials
- [ ] Click key icon next to a domain
- [ ] **Expected:** Modal with credentials
  - [ ] Username (UUID)
  - [ ] Password (40 chars)
  - [ ] Full domain name
  - [ ] Copy buttons

### 9. Delete Domain
- [ ] Click trash icon next to a domain
- [ ] **Expected:** Confirmation dialog
- [ ] Confirm deletion
- [ ] **Expected:** Domain removed from table

### 10. Admin Panel (/admin)
- [ ] Click "Admin" in navigation (admin users only)
- [ ] **Expected:** Admin panel page
  - [ ] Users tab
  - [ ] All Domains tab
  - [ ] Create User button
  - [ ] List of users
  - [ ] List of all domains

### 11. Admin - Create User
- [ ] Click "Create User"
- [ ] **Expected:** Modal dialog
- [ ] Fill in email and password
- [ ] Check "Admin user" checkbox (optional)
- [ ] Click "Create User"
- [ ] **Expected:**
  - [ ] User appears in list
  - [ ] Can login with new user

### 12. Admin Panel Without Admin Auth
- [ ] Login as regular user
- [ ] Navigate to `http://your-server/admin`
- [ ] **Expected:** 403 Forbidden error
- [ ] Should NOT show admin panel

### 13. Logout
- [ ] Click "Logout" in navigation menu
- [ ] **Expected:** Redirect to `/login`
- [ ] Session should be cleared
- [ ] Accessing `/dashboard` should redirect to login

### 14. Static Files
- [ ] Check browser dev tools → Network tab
- [ ] Static files should load:
  - [ ] `/static/css/style.css`
  - [ ] `/static/js/app.js`
  - [ ] Bootstrap from CDN
  - [ ] Bootstrap Icons from CDN

### 15. Mobile Responsive
- [ ] Test on mobile device or resize browser
- [ ] **Expected:**
  - [ ] Navigation collapses to hamburger menu
  - [ ] Forms are readable and usable
  - [ ] Tables are scrollable
  - [ ] No horizontal scrolling

## Automated Integration Tests

Run integration tests:
```bash
# With CGO enabled (requires MinGW on Windows)
set CGO_ENABLED=1
go test -v -tags=integration ./... -run TestWebUIIntegration
```

Run template tests:
```bash
go test -v -run TestTemplateRendering
```

## Common Issues and Solutions

### Issue: Login page shows just a newline
**Cause:** Templates not executing properly
**Fix:** Check that base.html exists and templates reference it correctly

### Issue: 404 on root /
**Cause:** Root route not registered
**Fix:** Check main.go has GET / route to RootHandler

### Issue: Static files 404
**Cause:** Embedded filesystem not working
**Fix:** Ensure web/static files exist and build includes embed

### Issue: Session not persisting
**Cause:** Cookie settings incorrect
**Fix:** Check secure cookie flag matches HTTPS usage

### Issue: CSRF token errors
**Cause:** CSRF middleware blocking requests
**Fix:** Check form includes csrf_token hidden field

## Browser Developer Tools Checks

### Console (F12 → Console)
- [ ] No JavaScript errors
- [ ] No 404 errors for resources
- [ ] No CSP violations

### Network (F12 → Network)
- [ ] All requests return expected status codes
- [ ] Static files load (200 OK)
- [ ] API calls work (register domain, etc.)
- [ ] Session cookie present after login

### Application (F12 → Application → Cookies)
- [ ] Session cookie set after login
- [ ] Cookie has correct name: `acmedns_session`
- [ ] Cookie has HttpOnly flag
- [ ] Cookie has Secure flag (if HTTPS)

## Security Checks

### Headers (curl or browser dev tools)
```bash
curl -I https://your-server/login
```

Should include:
- [ ] `X-Content-Type-Options: nosniff`
- [ ] `X-Frame-Options: DENY`
- [ ] `Content-Security-Policy: ...`
- [ ] `X-XSS-Protection: 1; mode=block`
- [ ] `Referrer-Policy: strict-origin-when-cross-origin`

### Rate Limiting
- [ ] Try 10 failed logins rapidly
- [ ] **Expected:** Rate limit error after configured limit
- [ ] Should receive 429 Too Many Requests

### CSRF Protection
- [ ] Try POST request without CSRF token
- [ ] **Expected:** 403 Forbidden
- [ ] Valid CSRF token allows request

## Performance Checks

### Page Load Time
- [ ] Login page loads < 2 seconds
- [ ] Dashboard loads < 2 seconds
- [ ] No unnecessary requests
- [ ] Resources cached properly

### Database Queries
- [ ] Dashboard makes minimal DB queries
- [ ] Admin panel paginates large lists (if implemented)
- [ ] No N+1 query problems

## Compatibility Testing

### Browsers
- [ ] Chrome/Edge (latest)
- [ ] Firefox (latest)
- [ ] Safari (latest)
- [ ] Mobile browsers

### Screen Sizes
- [ ] Desktop (1920x1080)
- [ ] Laptop (1366x768)
- [ ] Tablet (768x1024)
- [ ] Mobile (375x667)

## Deployment Verification

After deploying to production:

1. **Backup Current Database**
   ```bash
   cp /var/lib/acme-dns/acme-dns.db /var/lib/acme-dns/acme-dns.db.backup
   ```

2. **Test Existing API** (backward compatibility)
   ```bash
   curl -X POST https://your-server/register
   # Should still work exactly as before
   ```

3. **Create Test Admin User**
   ```bash
   ./acme-dns --create-admin test@example.com
   ```

4. **Verify Web UI Access**
   - Access https://your-server/login
   - Login with test admin
   - Register a test domain
   - Verify it works with certbot/acme clients

5. **Monitor Logs**
   ```bash
   journalctl -u acme-dns -f
   ```
   - Look for template errors
   - Check for authentication failures
   - Monitor database errors

## Rollback Plan

If web UI doesn't work:

1. **Disable Web UI**
   ```toml
   [webui]
   enabled = false
   ```

2. **Restart Service**
   ```bash
   systemctl restart acme-dns
   ```

3. **API Should Still Work**
   - Existing API endpoints unaffected
   - Existing domains continue to function

## Success Criteria

✅ All checklist items pass
✅ No console errors
✅ All automated tests pass
✅ Performance acceptable
✅ Works on multiple browsers
✅ Mobile responsive
✅ Security headers present
✅ Existing API still works

---

**Last Updated:** 2025-10-05
**Version:** 2.0.0
