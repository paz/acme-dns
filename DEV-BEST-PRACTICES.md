# Development Best Practices - acme-dns

## JSON API Endpoints - Critical Rules

### 1. Always Return JSON for JSON Endpoints

**❌ WRONG:**
```go
http.Error(w, "Invalid CSRF token", http.StatusForbidden)
```
This returns `Content-Type: text/plain` and breaks JavaScript JSON parsing.

**✅ CORRECT:**
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusForbidden)
w.Write([]byte(`{"status": "error", "message": "Invalid CSRF token"}`))
```

Or use encoding/json:
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusForbidden)
json.NewEncoder(w).Encode(map[string]string{
    "status": "error",
    "message": "Invalid CSRF token",
})
```

### 2. Standard Response Format

All JSON responses should follow this format:

**Success:**
```json
{
  "status": "success",
  "data": { ... }
}
```

**Error:**
```json
{
  "status": "error",
  "message": "Human-readable error message"
}
```

**Partial Success (Bulk Operations):**
```json
{
  "status": "success",
  "success_count": 5,
  "fail_count": 2,
  "total": 7,
  "errors": ["domain1: error message", "domain2: error message"]
}
```

## Middleware Best Practices

### 1. Order Matters

Middleware is executed in reverse order when chained:
```go
api.POST("/endpoint", web.ChainMiddleware(
    handler,
    CSRFMiddleware,      // Executes THIRD
    RequireAuth,         // Executes SECOND
    LoggingMiddleware,   // Executes FIRST
))
```

Common order:
1. LoggingMiddleware (outermost)
2. SecurityHeadersMiddleware
3. RateLimitMiddleware
4. RequireAuth / RequireAdmin
5. CSRFMiddleware
6. RequestSizeLimitMiddleware
7. Handler (innermost)

### 2. Setting Headers Before Writing Body

**❌ WRONG:**
```go
w.Write([]byte(`{"error": "message"}`))
w.Header().Set("Content-Type", "application/json") // Too late!
```

**✅ CORRECT:**
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusBadRequest)
w.Write([]byte(`{"error": "message"}`))
```

### 3. Error Handling in Middleware

Always return JSON errors for JSON endpoints:
```go
func MyMiddleware(next httprouter.Handle) httprouter.Handle {
    return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
        if someCondition {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusForbidden)
            w.Write([]byte(`{"status": "error", "message": "Forbidden"}`))
            return
        }
        next(w, r, ps)
    }
}
```

## CSRF Protection

### 1. Token in Template

Every authenticated page must include:
```html
<meta name="csrf-token" content="{{.CSRFToken}}">
```

### 2. Reading Token in JavaScript

```javascript
// At page load
let csrfToken = '';
document.addEventListener('DOMContentLoaded', () => {
    csrfToken = document.querySelector('meta[name="csrf-token"]')?.getAttribute('content') || '';
});
```

### 3. Sending Token with Requests

**For fetch requests:**
```javascript
fetch('/api/endpoint', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken
    },
    body: JSON.stringify(data)
})
```

**For form submissions:**
```javascript
const formData = new FormData(form);
fetch('/endpoint', {
    method: 'POST',
    headers: {
        'X-CSRF-Token': csrfToken
    },
    body: formData
})
```

### 4. CSRF Middleware Must Return JSON

```go
if formToken != csrfToken {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusForbidden)
    w.Write([]byte(`{"status": "error", "message": "Invalid CSRF token"}`))
    return
}
```

## Content Security Policy (CSP)

### 1. No Inline Scripts

**❌ WRONG:**
```html
<script>
    function doSomething() { ... }
</script>
```

**✅ CORRECT:**
```html
<script src="/static/js/app.js"></script>
```

### 2. No Inline Event Handlers

**❌ WRONG:**
```html
<button onclick="deleteItem()">Delete</button>
```

**✅ CORRECT:**
```html
<button class="delete-btn" data-item-id="123">Delete</button>

<script>
// In app.js
document.addEventListener('click', (e) => {
    if (e.target.closest('.delete-btn')) {
        const itemId = e.target.dataset.itemId;
        deleteItem(itemId);
    }
});
</script>
```

### 3. Event Delegation

Use event delegation for dynamically added elements:
```javascript
// ✅ Works for dynamic content
document.addEventListener('click', (e) => {
    if (e.target.closest('.dynamic-button')) {
        handleClick(e.target);
    }
});

// ❌ Won't work for elements added later
document.querySelectorAll('.dynamic-button').forEach(btn => {
    btn.addEventListener('click', handleClick);
});
```

## Go Template Best Practices

### 1. Unique Content Block Names

**❌ WRONG:**
```html
<!-- login.html -->
{{define "content"}}...{{end}}

<!-- dashboard.html -->
{{define "content"}}...{{end}}  <!-- Collision! -->
```

**✅ CORRECT:**
```html
<!-- login.html -->
{{define "login-content"}}...{{end}}

<!-- dashboard.html -->
{{define "dashboard-content"}}...{{end}}
```

### 2. Template Inheritance

```go
// render() function
contentBlockMap := map[string]string{
    "login.html":     "login-content",
    "dashboard.html": "dashboard-content",
}

tmpl, _ := h.templates.Clone()
tmpl, _ = tmpl.AddParseTree("content", h.templates.Lookup(contentBlockMap[templateName]).Tree)
tmpl.ExecuteTemplate(w, "base", data)
```

### 3. HTML Escaping

Templates automatically escape HTML, but be careful with:
```html
<!-- Automatically escaped -->
<div>{{.UserInput}}</div>

<!-- NOT escaped - dangerous! -->
<script>var data = {{.UserData}}</script>

<!-- Use this instead -->
<script>var data = JSON.parse('{{.UserDataJSON}}');</script>
```

## Database Best Practices

### 1. Always Use Parameterized Queries

**❌ WRONG (SQL Injection):**
```go
query := fmt.Sprintf("SELECT * FROM users WHERE email = '%s'", email)
db.Exec(query)
```

**✅ CORRECT:**
```go
query := "SELECT * FROM users WHERE email = $1"  // PostgreSQL
query := "SELECT * FROM users WHERE email = ?"   // SQLite
db.Exec(query, email)
```

### 2. Handle Both SQLite and PostgreSQL

```go
var query string
if Config.Database.Engine == "sqlite3" {
    query = "DELETE FROM table WHERE created < ?"
} else {
    query = "DELETE FROM table WHERE created < $1"
}
db.Exec(query, timestamp)
```

### 3. Migrations

- Always test migrations with existing data
- Support both SQLite and PostgreSQL
- Use transactions for atomic migrations
- Log all migration steps

## Error Handling

### 1. Don't Leak Sensitive Info

**❌ WRONG:**
```go
http.Error(w, err.Error(), http.StatusInternalServerError)
// Might expose: "database at /var/lib/acme-dns/db.sqlite failed"
```

**✅ CORRECT:**
```go
log.WithFields(log.Fields{"error": err}).Error("Database query failed")
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusInternalServerError)
w.Write([]byte(`{"status": "error", "message": "Internal server error"}`))
```

### 2. Log Detailed Errors, Return Generic Messages

```go
if err != nil {
    log.WithFields(log.Fields{
        "error":   err,
        "user_id": userID,
        "action":  "delete_domain",
    }).Error("Failed to delete domain")

    // Return generic message to user
    return fmt.Errorf("failed to delete domain")
}
```

## Testing Checklist

Before considering any feature complete:

1. [ ] Build succeeds: `go build -v`
2. [ ] Tests pass: `go test -v ./...`
3. [ ] Open browser console (F12)
4. [ ] Perform action
5. [ ] Check for JavaScript errors
6. [ ] Check Network tab for failed requests
7. [ ] Verify response format is JSON (if applicable)
8. [ ] Check for CSP violations
9. [ ] Test with rate limiting enabled
10. [ ] Test with CSRF protection

## JavaScript Best Practices

### 1. Fetch API Error Handling

```javascript
fetch('/api/endpoint', {
    method: 'POST',
    headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken
    },
    body: JSON.stringify(data)
})
.then(response => {
    if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
    }
    return response.json();
})
.then(data => {
    if (data.status === 'success') {
        showToast('Success!', 'success');
    } else {
        showToast(data.message || 'Operation failed', 'danger');
    }
})
.catch(error => {
    console.error('Error:', error);
    showToast('Network error occurred', 'danger');
});
```

### 2. Avoid Race Conditions

```javascript
// ❌ Can double-submit
button.addEventListener('click', async () => {
    await fetch('/api/endpoint', {...});
});

// ✅ Prevents double-submit
button.addEventListener('click', async (e) => {
    e.target.disabled = true;
    try {
        await fetch('/api/endpoint', {...});
    } finally {
        e.target.disabled = false;
    }
});
```

## Security Checklist

### Authentication
- [ ] Session ID is crypto-secure (32+ bytes)
- [ ] Sessions expire
- [ ] Can revoke sessions
- [ ] Logout destroys session

### Authorization
- [ ] Check user owns resource
- [ ] Admin-only routes protected
- [ ] CSRF protection on state-changing operations

### Input Validation
- [ ] Email validation
- [ ] Password complexity
- [ ] SQL injection prevention (parameterized queries)
- [ ] XSS prevention (template escaping)
- [ ] Request size limits

### Passwords
- [ ] Bcrypt for hashing (cost 10-12)
- [ ] Never log passwords
- [ ] Enforce minimum length
- [ ] Password reset tokens are secure

### Tokens
- [ ] CSRF tokens per session
- [ ] Password reset tokens are crypto-secure
- [ ] Tokens expire
- [ ] One-time use for sensitive operations

## Common Mistakes and Fixes

### Mistake #1: Text Errors for JSON Endpoints
**Symptom**: "SyntaxError: Unexpected token" in browser console

**Fix**: Always return JSON from JSON endpoints

### Mistake #2: Headers After Body
**Symptom**: Headers not being set

**Fix**: Set headers before calling `WriteHeader()` or `Write()`

### Mistake #3: Missing CSRF Token
**Symptom**: All POST/DELETE requests fail with 403

**Fix**: Include `<meta name="csrf-token">` and read in JavaScript

### Mistake #4: Inline Scripts
**Symptom**: CSP violations in console

**Fix**: Move all scripts to external files, use event delegation

### Mistake #5: Template Name Collision
**Symptom**: Wrong content displays

**Fix**: Use unique content block names

## Git Commit Best Practices

1. Build must succeed before committing
2. One logical change per commit
3. Descriptive commit messages
4. Reference issue/bug if applicable
5. Test the change in browser

## Code Review Checklist

- [ ] JSON responses have correct Content-Type
- [ ] CSRF protection on state-changing operations
- [ ] No inline scripts or event handlers
- [ ] Error messages don't leak sensitive info
- [ ] Parameterized queries (no SQL injection)
- [ ] Both SQLite and PostgreSQL supported
- [ ] Logging includes context (user ID, action, etc.)
- [ ] Browser console checked for errors

---

*Keep this document updated as new patterns emerge*

## Email Templates

### 1. Sending Password Reset Emails

When sending password reset emails, use HTML templates with proper formatting:

**Example from admin.CreateUser:**
```go
// Create password reset token
resetObj, err := h.passwordResetRepo.Create(newUser.ID, email, 24)
if err \!= nil {
    // Handle error
}

// Send password reset email
resetURL := fmt.Sprintf("%s/reset-password?token=%s", h.baseURL, resetObj.Token)
subject := "Set Your Password - acme-dns"
body := fmt.Sprintf(`
    <html>
    <body>
        <h2>Welcome to acme-dns\!</h2>
        <p>An administrator has created an account for you. Please set your password by clicking the link below:</p>
        <p><a href="%s">Set Password</a></p>
        <p>This link will expire in 24 hours.</p>
        <p>If you did not request this account, please ignore this email.</p>
    </body>
    </html>
`, resetURL)

if err := h.mailer.SendEmail(email, subject, body); err \!= nil {
    // Handle error
}
```

### 2. Email Template Best Practices

- ✅ Use HTML for better formatting
- ✅ Include clear call-to-action (link or button)
- ✅ Specify expiration time for tokens
- ✅ Include security notice (ignore if not requested)
- ✅ Use baseURL from config for consistent links
- ✅ Log email failures but continue operation if possible

