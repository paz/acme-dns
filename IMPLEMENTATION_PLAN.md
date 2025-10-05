# acme-dns Security, Performance & Web UI Implementation Plan

## Security, Performance & Best Practices Review

### Security Findings

**Strengths:**
- ✅ Bcrypt password hashing (cost 10)
- ✅ Timing attack protection (auth.go:70)
- ✅ SQL injection prevention via prepared statements
- ✅ TLS 1.2 minimum version
- ✅ Crypto-secure random password generation
- ✅ Optional CIDR-based IP restrictions
- ✅ File permissions (umask 0077)

**Issues & Recommendations:**

1. **No Rate Limiting** - HIGH PRIORITY
   - Missing on `/register` and `/update` endpoints
   - Vulnerable to DoS and brute force attacks
   - Recommendation: Add rate limiting middleware (e.g., golang.org/x/time/rate)

2. **Information Disclosure** - MEDIUM
   - Error messages reveal valid usernames (auth.go:77)
   - Recommendation: Use generic "Invalid credentials" message

3. **No Request Size Limits** - MEDIUM
   - Could be exploited with large payloads
   - Recommendation: Add `http.MaxBytesReader` to limit request body size

4. **CORS Too Permissive** - MEDIUM
   - Default config allows "*" origins
   - Recommendation: Document security implications, suggest specific domains

5. **No Admin Authentication** - HIGH (for web UI)
   - No way to authenticate administrators
   - Required for web UI implementation

6. **No Session Management** - HIGH (for web UI)
   - Currently stateless header-based auth
   - Need secure session handling for browser-based UI

7. **Missing Security Headers** - LOW
   - No CSP, X-Frame-Options, etc.
   - Recommendation: Add security headers middleware

### Performance Findings

**Issues & Recommendations:**

1. **Global Database Mutex** - HIGH PRIORITY
   - Single mutex locks entire database (db.go:58, 174, 213, 251, 285)
   - Serializes all database operations
   - Recommendation: Use row-level locking or separate read/write locks

2. **No Database Connection Pooling** - MEDIUM
   - sql.DB supports pooling but not configured
   - Recommendation: Set `SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime`

3. **Missing Database Indexes** - MEDIUM
   - `txt` table queried by Subdomain without explicit index
   - Recommendation: Add index on `txt(Subdomain, LastUpdate)`

4. **No Caching** - LOW
   - DNS responses could be cached briefly
   - Recommendation: Add short-lived in-memory cache for frequent lookups

5. **No Graceful Shutdown** - MEDIUM
   - main.go:96-101 blocks forever, no signal handling
   - Recommendation: Handle SIGTERM/SIGINT for graceful shutdown

6. **No Metrics/Monitoring** - LOW
   - No prometheus or similar metrics
   - Recommendation: Add metrics for registration, updates, DNS queries

### Best Practices Findings

**Issues & Recommendations:**

1. **Magic Numbers** - LOW
   - TXT length "43" hardcoded (validation.go:36)
   - Key length "40" hardcoded (validation.go:21)
   - Recommendation: Define as constants

2. **Manual JSON Construction** - LOW
   - api.go:102 manually constructs JSON
   - Recommendation: Use struct marshalling

3. **Health Check Too Basic** - LOW
   - Just returns 200, doesn't check database
   - Recommendation: Add database ping to health check

4. **Inconsistent Error Handling** - LOW
   - Some errors are too generic
   - Recommendation: Define custom error types

5. **Missing Godoc Comments** - LOW
   - Many exported functions lack documentation
   - Recommendation: Add comprehensive godoc comments

---

## Web UI Implementation Plan

### Overview

Add a full-featured web UI to acme-dns with user accounts, authentication, domain management, and admin dashboard while maintaining backward compatibility with the existing API.

### Architecture

```
┌─────────────────────────────────────────────────┐
│                  Web Browser                    │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│            HTTP/HTTPS Server                     │
│  ┌──────────────┐  ┌──────────────────────────┐ │
│  │  Static      │  │   API Routes             │ │
│  │  Files       │  │   /api/...              │ │
│  │  /ui/...     │  │                          │ │
│  └──────────────┘  └──────────────────────────┘ │
│  ┌──────────────────────────────────────────┐   │
│  │   Web UI Routes (Session-based)          │   │
│  │   /login, /dashboard, /admin             │   │
│  └──────────────────────────────────────────┘   │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│          Authentication Layer                    │
│  ┌──────────────┐  ┌──────────────────────────┐ │
│  │  Session     │  │   API Key (existing)     │ │
│  │  Middleware  │  │   Middleware             │ │
│  └──────────────┘  └──────────────────────────┘ │
└─────────────────┬───────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────┐
│              Database Layer                      │
│  ┌────────────┬──────────┬────────────────────┐ │
│  │   users    │ sessions │  records (existing)│ │
│  │   (new)    │  (new)   │  txt (existing)    │ │
│  └────────────┴──────────┴────────────────────┘ │
└─────────────────────────────────────────────────┘
```

### Phase 1: Database Schema Extensions

**New Tables:**

```sql
-- User accounts table
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,  -- or SERIAL for postgres
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    last_login INTEGER,
    active BOOLEAN NOT NULL DEFAULT 1
);

-- Sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Link records to user accounts
ALTER TABLE records ADD COLUMN user_id INTEGER;
ALTER TABLE records ADD COLUMN created_at INTEGER;
ALTER TABLE records ADD COLUMN description TEXT;  -- optional: user-friendly name
```

**Migration Strategy:**
- Create new file: `db_migrations.go`
- Version database schema (currently at v1, move to v2)
- Existing records get `user_id = NULL` (API-only access, backward compatible)
- Admin can claim/manage unclaimed records

### Phase 2: Backend Infrastructure

**File Structure:**
```
.
├── web/
│   ├── handlers.go       # Web UI handlers
│   ├── session.go        # Session management
│   ├── middleware.go     # Auth middleware for web
│   ├── templates/        # HTML templates
│   │   ├── layout.html
│   │   ├── login.html
│   │   ├── dashboard.html
│   │   ├── admin.html
│   │   └── components/
│   └── static/           # CSS, JS, images
│       ├── css/
│       ├── js/
│       └── img/
├── models/
│   ├── user.go          # User model
│   └── session.go       # Session model
├── auth/
│   ├── session_auth.go  # Session-based auth
│   └── password.go      # Password utilities
└── admin/
    └── handlers.go      # Admin-specific handlers
```

**New Files to Create:**

1. **models/user.go** - User account management
```go
type User struct {
    ID           int64
    Email        string
    PasswordHash string
    IsAdmin      bool
    CreatedAt    time.Time
    LastLogin    *time.Time
    Active       bool
}

// Methods: Create, GetByEmail, GetByID, Update, Delete, ChangePassword, etc.
```

2. **models/session.go** - Session management
```go
type Session struct {
    ID        string
    UserID    int64
    CreatedAt time.Time
    ExpiresAt time.Time
    IPAddress string
    UserAgent string
}

// Methods: Create, Get, Delete, Cleanup (remove expired), etc.
```

3. **web/session.go** - Session handling with secure cookies
```go
- Session creation with crypto/rand
- HTTP-only, Secure, SameSite cookies
- CSRF token generation/validation
- Session expiration (e.g., 24 hours)
```

4. **web/middleware.go** - Authentication middleware
```go
- RequireAuth: Check session validity
- RequireAdmin: Check admin privileges
- CSRFProtection: Validate CSRF tokens
- RateLimiter: Prevent abuse
```

5. **web/handlers.go** - Web UI handlers
```go
- LoginPage, LoginHandler, LogoutHandler
- DashboardHandler
- RegisterDomainHandler (web version)
- UpdateDomainHandler
- DeleteDomainHandler
- ProfileHandler
```

6. **admin/handlers.go** - Admin-specific handlers
```go
- AdminDashboardHandler
- ListAllUsersHandler
- ListAllDomainsHandler
- CreateUserHandler
- UpdateUserHandler
- DeleteUserHandler
- ClaimUnmanagedDomainHandler
```

### Phase 3: Frontend Design

**Technology Stack:**
- HTML5 + Go templates (`html/template`)
- CSS: Tailwind CSS or Bootstrap 5 (via CDN for simplicity)
- JavaScript: Vanilla JS or Alpine.js (lightweight)
- Optional: HTMX for dynamic updates without heavy JS framework

**Pages:**

1. **Login Page** (`/login`)
   - Email/password form
   - CSRF protection
   - Remember me option
   - Password reset link (future)

2. **User Dashboard** (`/dashboard`)
   - List user's registered domains
   - Table showing: Subdomain, Full Domain, Created Date, Last Updated
   - Actions: Update TXT, View Credentials, Delete
   - Button: Register New Domain
   - Copy credentials to clipboard feature

3. **Register Domain Modal/Page** (`/dashboard/register`)
   - Form with optional CIDR restrictions
   - Optional description field
   - Generate credentials button
   - Display credentials (one-time view warning)

4. **Admin Dashboard** (`/admin`)
   - Statistics: Total users, total domains, recent activity
   - Search/filter capabilities
   - Tabs:
     - Users: List, create, edit, delete users
     - Domains: List all domains, search by user/domain
     - Unmanaged: API-created records without user_id
     - Activity Log (future)

5. **Profile Page** (`/profile`)
   - Change email
   - Change password
   - View login history
   - Logout from all sessions

**UI/UX Considerations:**
- Responsive design (mobile-friendly)
- Dark mode support
- Accessibility (WCAG 2.1 AA)
- Clear warning when showing API credentials
- Confirmation dialogs for destructive actions

### Phase 4: API Enhancements

**New API Endpoints:**

```
POST   /api/v1/auth/login            - Login (returns session cookie)
POST   /api/v1/auth/logout           - Logout
GET    /api/v1/auth/me               - Get current user info

GET    /api/v1/domains               - List user's domains
POST   /api/v1/domains               - Register new domain (web UI version)
GET    /api/v1/domains/:id           - Get domain details
DELETE /api/v1/domains/:id           - Delete domain
PUT    /api/v1/domains/:id/txt       - Update TXT record

# Admin endpoints
GET    /api/v1/admin/users           - List all users
POST   /api/v1/admin/users           - Create user
GET    /api/v1/admin/users/:id       - Get user details
PUT    /api/v1/admin/users/:id       - Update user
DELETE /api/v1/admin/users/:id       - Delete user

GET    /api/v1/admin/domains         - List all domains
GET    /api/v1/admin/domains/:id     - Get domain details
DELETE /api/v1/admin/domains/:id     - Delete any domain
POST   /api/v1/admin/domains/:id/claim - Claim unmanaged domain
```

**Maintain Backward Compatibility:**
- Keep existing `/register` and `/update` endpoints
- These work without user accounts (user_id = NULL)
- Can be disabled separately from web registration

### Phase 5: Security Enhancements

**Implement:**

1. **Rate Limiting**
   - Use `golang.org/x/time/rate`
   - Per-IP limits on login attempts (5/minute)
   - Per-user limits on domain registration (10/hour)
   - Per-IP limits on API registration (3/minute)

2. **Session Security**
   - HTTP-only cookies
   - Secure flag (HTTPS only)
   - SameSite=Strict
   - Random session IDs (crypto/rand)
   - Session expiration and cleanup

3. **CSRF Protection**
   - Generate CSRF token per session
   - Validate on all state-changing operations
   - Use double-submit cookie pattern

4. **Password Requirements**
   - Minimum 12 characters
   - Complexity requirements
   - Check against common passwords list
   - Bcrypt with cost 12 (higher than API keys)

5. **Security Headers**
```go
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Content-Security-Policy: default-src 'self'
Strict-Transport-Security: max-age=31536000
```

6. **Input Validation**
   - Email validation
   - Sanitize all user inputs
   - Limit request sizes

7. **Audit Logging**
   - Log authentication attempts
   - Log administrative actions
   - Optional: Store in separate `audit_log` table

### Phase 6: Configuration Updates

**Add to config.cfg:**

```toml
[webui]
# Enable/disable web UI
enabled = true
# Session duration in hours
session_duration = 24
# Require email verification (future)
require_email_verification = false
# Allow user self-registration (vs admin-only user creation)
allow_self_registration = true
# Minimum password length
min_password_length = 12

[security]
# Enable rate limiting
rate_limiting = true
# Max login attempts before lockout
max_login_attempts = 5
# Lockout duration in minutes
lockout_duration = 15
# Session cookie name
session_cookie_name = "acmedns_session"
# CSRF cookie name
csrf_cookie_name = "acmedns_csrf"
```

### Phase 7: Implementation Steps

**Step-by-step implementation order:**

1. **Database Layer** (2-3 days)
   - Create migration system
   - Add new tables (users, sessions)
   - Extend records table
   - Update db.go with new methods
   - Write comprehensive tests

2. **User & Session Models** (1-2 days)
   - Implement user CRUD operations
   - Implement session management
   - Password hashing/validation
   - Tests for all operations

3. **Web Authentication** (2-3 days)
   - Session middleware
   - CSRF protection
   - Login/logout handlers
   - Rate limiting middleware
   - Security headers middleware

4. **Web UI - Basic Pages** (3-4 days)
   - Template system setup
   - Login page
   - User dashboard (read-only)
   - Static assets structure
   - Responsive layout

5. **Domain Management UI** (2-3 days)
   - Register domain form
   - Update TXT record
   - Delete domain
   - View credentials

6. **Admin Dashboard** (3-4 days)
   - User management CRUD
   - Domain listing/search
   - Statistics dashboard
   - Claim unmanaged domains

7. **API Enhancements** (1-2 days)
   - RESTful API endpoints
   - JSON responses
   - API documentation

8. **Testing & Polish** (2-3 days)
   - Integration tests
   - UI/UX refinement
   - Security audit
   - Documentation

**Total Estimated Time: 16-24 days**

### Phase 8: Testing Strategy

**Test Coverage:**

1. **Unit Tests**
   - User model operations
   - Session management
   - Password validation
   - CSRF token generation/validation

2. **Integration Tests**
   - Login flow
   - Domain registration flow
   - Admin operations
   - API backward compatibility

3. **Security Tests**
   - SQL injection attempts
   - XSS attempts
   - CSRF attacks
   - Session hijacking
   - Rate limiting effectiveness

4. **E2E Tests** (optional)
   - Use selenium/playwright
   - Test complete user journeys

### Phase 9: Documentation

**Update Documentation:**

1. **README.md**
   - Add Web UI section
   - Installation instructions for web UI
   - Default admin account creation

2. **WEB_UI.md** (new)
   - User guide
   - Screenshots
   - Feature overview
   - FAQ

3. **ADMIN_GUIDE.md** (new)
   - Admin panel guide
   - User management
   - Security best practices

4. **API.md** (new)
   - API documentation
   - Authentication methods
   - Endpoint reference
   - Migration guide from old API

### Phase 10: Deployment Considerations

**Docker Updates:**
- Update Dockerfile to include web assets
- Add volume for session storage (if file-based)
- Update docker-compose.yml with new config

**Systemd Service:**
- No changes needed (backward compatible)

**Initial Admin Account:**
- Create via CLI command: `acme-dns --create-admin`
- Or automatic on first run with env vars

**Migration Path:**
- Existing deployments: Web UI disabled by default
- Enable via config
- Run migration command to update database
- Create admin account
- Optionally claim existing API-created domains

### Technology Choices

**Required Libraries:**

```go
github.com/gorilla/sessions      // Session management
github.com/gorilla/csrf          // CSRF protection
golang.org/x/time/rate          // Rate limiting
golang.org/x/crypto/bcrypt      // Already used
```

**Optional Libraries:**

```go
github.com/gorilla/securecookie  // Included with gorilla/sessions
github.com/microcosm-cc/bluemonday // HTML sanitization
github.com/go-playground/validator  // Struct validation
```

### Migration Guide for Existing Users

1. Backup database
2. Update acme-dns binary
3. Run database migration: `acme-dns --migrate`
4. Create admin account: `acme-dns --create-admin email@example.com`
5. Enable web UI in config
6. Restart service
7. Access web UI at `https://your-domain/login`
8. Optionally claim existing API-created domains

---

## Summary

This plan provides a comprehensive web UI for acme-dns while:
- ✅ Maintaining 100% backward compatibility with existing API
- ✅ Adding modern session-based authentication
- ✅ Implementing proper security controls
- ✅ Providing user-friendly domain management
- ✅ Enabling administrative oversight
- ✅ Following Go and web security best practices
- ✅ Supporting both SQLite and PostgreSQL
- ✅ Maintaining the project's simplicity philosophy

The implementation is phased to allow incremental development and testing, with each phase building on the previous one.
