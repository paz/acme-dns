# Testing Guide for Template Fixes

## Critical Fixes Applied

### 1. Template Syntax Error (FIXED ✅)
**Error:** `template: layout.html:67: unexpected ".Data" in template clause"`
**Fix:** Removed dynamic template name lookup, simplified to `{{template "content" .}}`
**Commit:** c8991f8

### 2. Route Conflict (FIXED ✅)
**Error:** `panic: a handle is already registered for path '/register'`
**Fix:** Renamed web UI registration to `/signup` to avoid conflict with API `/register`
**Commit:** dbd7bda

### 3. Template Content Block Naming (NEEDS VERIFICATION ⚠️)
**Issue:** All templates define `{{define "content"}}` with same name
**Result:** When parsed together, only last alphabetically (register.html) is used
**Status:** Needs testing on production server

## Testing on Production Server

### Step 1: Pull Latest Changes
```bash
cd /opt/acme-dns
git pull origin master

# Should show these commits:
# - c8991f8 Fix template rendering to use correct Go template pattern
# - dbd7bda Fix duplicate /register route conflict
# - e99fa07 Consolidate documentation and archive historical files
```

### Step 2: Rebuild
```bash
# With CGO for SQLite support
CGO_ENABLED=1 go build -v

# Should complete without errors
```

### Step 3: Start Server
```bash
# Stop existing instance
sudo systemctl stop acme-dns

# Start with your production config
./acme-dns -c /etc/acme-dns/config.cfg

# Check logs - should see:
# INFO Web UI enabled - initializing web components
# INFO Web UI routes registered successfully
# INFO Listening HTTP host="0.0.0.0:443"
```

### Step 4: Test Template Rendering

Test each page to verify correct content:

#### Test 1: Root Redirect
```bash
curl -I https://auth.busictgroup.com.au/
# Should: 303 redirect to /login
```

#### Test 2: Login Page
```bash
curl -s https://auth.busictgroup.com.au/login | grep "card-title"
# Should show: <i class="bi bi-shield-lock"></i> Login
# Should NOT show: Create Account or Register
```

#### Test 3: Dashboard (requires login)
```bash
# After logging in with admin credentials
# Should show: <i class="bi bi-speedometer2"></i> Dashboard
# Should have navigation bar with Dashboard, Admin, Profile links
```

#### Test 4: Profile Page
```bash
# When logged in
curl -s https://auth.busictgroup.com.au/profile -H "Cookie: acmedns_session=..."
# Should show: <i class="bi bi-person-circle"></i> Profile
# Should have password change form
```

#### Test 5: Admin Panel (admin only)
```bash
# When logged in as admin
# Should show: <i class="bi bi-gear"></i> Admin Dashboard
# Should have user management interface
```

#### Test 6: Registration Page
```bash
curl -s https://auth.busictgroup.com.au/signup | grep "card-title"
# Should show: <i class="bi bi-person-plus"></i> Create Account
# Note: Changed from /register to /signup
```

## Expected Results

### ✅ SUCCESS Indicators:
1. Server starts without template errors
2. No route panic on startup
3. Each page shows its own unique content (not all showing same page)
4. Login form has email/password fields
5. Dashboard shows domain list
6. Profile shows password change + sessions
7. Admin shows user/domain management

### ❌ FAILURE Indicators:
1. Template parse errors on startup
2. All pages show same content (likely register.html)
3. Route panic: "path already registered"
4. 404 errors on /login, /dashboard, /profile, /admin

## If Templates Still Show Wrong Content

If all pages are still showing the registration form, the issue is that all templates define the same `{{define "content"}}` block name. Fix by either:

### Option A: Give each template a unique block name
```go
// login.html
{{define "login-content"}}...{{end}}

// dashboard.html
{{define "dashboard-content"}}...{{end}}
```

Then update handlers to execute the specific template by name.

### Option B: Use template cloning (better approach)
Update `web/embed.go` GetTemplates() function to parse each page template separately and clone the base layout for each.

## API Testing (Should Still Work)

The existing API should be 100% backward compatible:

```bash
# Test domain registration
curl -X POST https://auth.busictgroup.com.au/register \
  -H "Content-Type: application/json" \
  -d '{"allowfrom": ["0.0.0.0/0"]}'

# Should return:
# {"username": "uuid", "password": "...", "fulldomain": "..."}
```

## Rollback If Needed

If there are critical issues:

```bash
# Revert to previous version
git checkout e99fa07  # Before template fixes
CGO_ENABLED=1 go build -v
sudo systemctl restart acme-dns
```

## Report Issues

If you encounter any failures, please provide:
1. Server startup logs (first 50 lines)
2. Output of curl tests above
3. Browser screenshot of pages showing wrong content

The template content block naming issue may require one more fix to properly support multiple pages with the same "content" block name.
