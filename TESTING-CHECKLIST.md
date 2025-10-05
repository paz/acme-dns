# Testing Checklist - acme-dns Web UI

This document provides a comprehensive testing checklist to ensure all functionality works correctly before deployment.

## Pre-Deployment Testing

### 1. Build and Compilation
- [ ] `go build -v` completes without errors
- [ ] `go test -v ./...` all tests pass
- [ ] No compiler warnings

### 2. Browser Console Checks
**Critical**: Always check browser console (F12) for errors before considering a feature complete.

**Common Issues to Check:**
- [ ] No JavaScript errors
- [ ] No CSP (Content Security Policy) violations
- [ ] No failed network requests (check Network tab)
- [ ] No "Refused to execute inline script" errors
- [ ] No "SyntaxError" when parsing JSON responses

### 3. Authentication & Sessions

#### Login
- [ ] Login page loads without errors
- [ ] Can log in with valid credentials
- [ ] Shows error message for invalid credentials
- [ ] "Forgot Password?" link works
- [ ] Redirects to intended page after login
- [ ] Session cookie is set correctly

#### Logout
- [ ] Logout button works
- [ ] Session is properly destroyed
- [ ] Redirects to login page
- [ ] Cannot access authenticated pages after logout

### 4. Dashboard (User Features)

#### Domain Registration
- [ ] "Register New Domain" button works
- [ ] Form appears when clicked
- [ ] Can register domain successfully
- [ ] Domain appears in list immediately
- [ ] **Check browser console for errors**

#### Domain Management
- [ ] Can view domain credentials
- [ ] Credentials modal shows correct info
- [ ] Can copy credentials
- [ ] Can delete own domains
- [ ] Deletion requires confirmation
- [ ] Domain removed from list after deletion

#### Domain Display
- [ ] Shows subdomain correctly
- [ ] Shows full domain correctly
- [ ] Shows description if set
- [ ] Shows creation date if available

### 5. Profile Page

#### Session Management
- [ ] Current session is highlighted
- [ ] Shows all active sessions
- [ ] Can revoke other sessions
- [ ] Cannot revoke current session
- [ ] Session list updates after revocation
- [ ] **Check browser console for 403 errors**

#### Password Change
- [ ] Password change form works
- [ ] Validates password length
- [ ] Validates password confirmation match
- [ ] Shows success message on change
- [ ] Can log in with new password
- [ ] **Check browser console for CSRF errors**

### 6. Admin Panel

#### User Management
- [ ] Shows all users
- [ ] Can create new user
- [ ] Email validation works
- [ ] Password requirements enforced
- [ ] Can set admin flag
- [ ] Can enable/disable users
- [ ] Can delete users
- [ ] **Check browser console for JSON parse errors**

#### Domain Management (All Domains Tab)
- [ ] Shows all domains (managed and unmanaged)
- [ ] Shows owner information
- [ ] Select-all checkbox works
- [ ] Individual checkboxes work
- [ ] Selected count updates
- [ ] Bulk delete button enables when selected
- [ ] Bulk delete works
- [ ] Confirmation dialog appears
- [ ] Shows success/fail counts

#### Unmanaged Domains Tab
- [ ] Shows only API-only domains
- [ ] Select-all checkbox works
- [ ] Bulk claim button enables when selected
- [ ] Bulk claim modal appears
- [ ] User dropdown populated
- [ ] Can set description
- [ ] Bulk claim works
- [ ] Shows success/fail counts
- [ ] **Critical: Check for 403 Forbidden errors**

### 7. Password Reset

#### Request Reset
- [ ] "Forgot Password?" link on login page
- [ ] Request form loads
- [ ] Can submit email address
- [ ] Doesn't reveal if email exists (same message for all)
- [ ] Email is sent (check logs)
- [ ] Email contains valid reset link

#### Reset Password
- [ ] Reset link works
- [ ] Shows form with password fields
- [ ] Invalid/expired token shows error
- [ ] Password validation works
- [ ] Password confirmation required
- [ ] Token marked as used after reset
- [ ] Can log in with new password
- [ ] Token can't be reused

### 8. API Endpoints (Backward Compatibility)

#### Domain Registration
- [ ] `POST /register` works without authentication
- [ ] Returns username, password, subdomain, fulldomain
- [ ] Can specify allowfrom CIDR ranges
- [ ] Domain appears in database

#### TXT Record Update
- [ ] `POST /update` with API key works
- [ ] X-Api-User and X-Api-Key headers validated
- [ ] TXT record is updated
- [ ] Returns updated TXT value

#### Health Check
- [ ] `GET /health` returns 200 OK
- [ ] Database is checked

### 9. CSRF Protection

**Critical: All state-changing operations must pass CSRF validation**

- [ ] All POST requests include CSRF token
- [ ] All DELETE requests include CSRF token
- [ ] CSRF token in meta tag on all authenticated pages
- [ ] JavaScript reads token from meta tag
- [ ] Requests fail without valid token
- [ ] **CSRF errors return JSON, not plain text**

**Test each endpoint:**
- [ ] Domain registration
- [ ] Domain deletion
- [ ] Bulk claim
- [ ] Bulk delete
- [ ] User creation
- [ ] User deletion
- [ ] User enable/disable
- [ ] Session revocation
- [ ] Password change
- [ ] Password reset request

### 10. Security Headers

Check response headers (Network tab → select request → Headers):
- [ ] `Content-Security-Policy` present
- [ ] `X-Content-Type-Options: nosniff` present
- [ ] `X-Frame-Options: DENY` present
- [ ] `Strict-Transport-Security` present (if HTTPS)
- [ ] `X-XSS-Protection: 1; mode=block` present

### 11. Rate Limiting

- [ ] Multiple rapid requests trigger rate limit
- [ ] Rate limit returns 429 status
- [ ] Rate limit resets after time period

### 12. Database Migrations

**Fresh Install (v0 → v2):**
- [ ] Database created successfully
- [ ] All tables created
- [ ] Indexes created
- [ ] No errors in logs

**Upgrade (v1 → v2):**
- [ ] Existing data preserved
- [ ] New tables created
- [ ] New columns added to records table
- [ ] Indexes created
- [ ] No errors in logs

### 13. Email System (if enabled)

- [ ] SMTP configuration in config.cfg
- [ ] Email enabled flag set
- [ ] Password reset email sent
- [ ] Email contains valid reset link
- [ ] Email HTML renders correctly
- [ ] From address and name correct

**Test Email Types:**
- [ ] Password reset email
- [ ] Welcome email (if implemented)
- [ ] Test email (if implemented)

## Browser Testing Matrix

Test in multiple browsers to ensure compatibility:
- [ ] Chrome/Edge (latest)
- [ ] Firefox (latest)
- [ ] Safari (if available)

## Network Condition Testing

- [ ] Works on slow connections
- [ ] Forms don't double-submit
- [ ] Loading states shown appropriately

## Error Scenarios

### Expected Failures
- [ ] Invalid credentials shows error
- [ ] Expired session redirects to login
- [ ] Invalid CSRF token shows error (JSON format!)
- [ ] Missing required fields shows validation error
- [ ] Duplicate email shows error
- [ ] Password too short shows error

### Error Response Format
**All errors must return JSON format:**
```json
{
  "status": "error",
  "message": "Human-readable error message"
}
```

**Never return plain text errors from JSON endpoints!**

## Logging and Monitoring

- [ ] Check application logs for errors
- [ ] Verify INFO level logs for successful operations
- [ ] Verify WARN level logs for security events
- [ ] Verify ERROR level logs have stack traces
- [ ] No sensitive data in logs (passwords, tokens)

## Critical Issues Found Previously

### CSRF Middleware Bug (Fixed in commit 47a27a4)
**Symptom**: All POST/DELETE requests return 403 Forbidden with "SyntaxError: Unexpected token 'C'"

**Cause**: CSRF middleware returned plain text errors instead of JSON

**Fix**: Changed all `http.Error()` calls to return JSON:
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusForbidden)
w.Write([]byte(`{"status": "error", "message": "Invalid CSRF token"}`))
```

**Prevention**:
- Always set `Content-Type: application/json` for JSON endpoints
- Test browser console for parse errors
- Check Network tab for response format

### Template Content Block Collision (Fixed earlier)
**Symptom**: All pages show registration form

**Cause**: All templates defined `{{define "content"}}` with same name

**Fix**: Unique content block names per template

**Prevention**:
- Test each route independently
- Verify correct content loads
- Check template parsing in tests

## Documentation

After testing, update:
- [ ] README.md with any new features
- [ ] CLAUDE.md with implementation notes
- [ ] TESTING-CHECKLIST.md (this file) with new findings
- [ ] User-facing documentation

## Pre-Deployment Checklist

Before deploying to production:
1. [ ] All tests in this checklist pass
2. [ ] No errors in browser console
3. [ ] No errors in application logs
4. [ ] Database backup created
5. [ ] Rollback plan documented
6. [ ] Configuration reviewed
7. [ ] Email settings tested (if enabled)
8. [ ] Admin user created
9. [ ] Health check endpoint verified
10. [ ] CSRF protection verified on all endpoints

## Post-Deployment Verification

After deploying:
1. [ ] Login works
2. [ ] Dashboard loads
3. [ ] Can perform one domain operation
4. [ ] Admin panel accessible
5. [ ] No 403 errors in browser console
6. [ ] Application logs clean
7. [ ] Health check returns 200 OK

---

## Quick Browser Console Test Commands

```javascript
// Check if CSRF token is available
console.log(document.querySelector('meta[name="csrf-token"]')?.getAttribute('content'));

// Check for CSP violations
// (These should appear automatically in console if present)

// Check all network requests
// (Open Network tab before performing actions)
```

## Common Gotchas

1. **CSRF Token Not Found**
   - Ensure meta tag in layout.html: `<meta name="csrf-token" content="{{.CSRFToken}}">`
   - JavaScript must read from meta tag, not inline variable

2. **403 Forbidden on All POST/DELETE**
   - CSRF middleware returning wrong format
   - Check Content-Type header is `application/json`
   - Verify JSON format: `{"status": "error", "message": "..."}`

3. **JSON Parse Errors**
   - Response is not valid JSON
   - Check response in Network tab
   - Often caused by `http.Error()` instead of JSON response

4. **CSP Violations**
   - No inline scripts allowed
   - All JavaScript must be in external files
   - No `onclick=""` attributes
   - Use event delegation instead

---

*Last Updated: 2025-10-05*
*Update this document when new issues are discovered*
