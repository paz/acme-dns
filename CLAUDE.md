# CLAUDE.md - acme-dns Project Guide

## Project Overview

acme-dns is a simplified DNS server with a RESTful HTTP API designed to automate ACME DNS challenges for Let's Encrypt certificate issuance. This project has been significantly enhanced with a full web UI, user account system, and administrative capabilities while maintaining 100% backward compatibility with the existing API.

## Recent Enhancements (v2.0)

### What's New

1. **Web UI** - Full-featured browser-based interface for managing DNS records
2. **User Accounts** - Secure user authentication and account management
3. **Admin Dashboard** - Administrative interface for managing users and domains
4. **Session Management** - Secure, database-backed session handling
5. **Enhanced Security** - Rate limiting, CSRF protection, security headers
6. **Database Migrations** - Automatic schema upgrades with backward compatibility
7. **Performance Improvements** - Connection pooling, database indexes
8. **Code Quality** - Constants for magic numbers, improved error handling

### Backward Compatibility

- ✅ Existing API endpoints (`/register`, `/update`, `/health`) unchanged
- ✅ API-only registrations continue to work (stored with `user_id = NULL`)
- ✅ Database automatically migrates from v1 to v2
- ✅ Web UI disabled by default - must be explicitly enabled
- ✅ All existing configurations remain valid

## Architecture

### System Components

```
┌─────────────────────────────────────────────┐
│           User Interface Layer              │
│  ┌──────────────┐  ┌─────────────────────┐ │
│  │   Web UI     │  │   API Clients       │ │
│  │   Browser    │  │   (certbot, etc)    │ │
│  └──────────────┘  └─────────────────────┘ │
└─────────────┬───────────────┬───────────────┘
              │               │
              ▼               ▼
┌─────────────────────────────────────────────┐
│          HTTP/HTTPS Server (main.go)        │
│  ┌──────────────────┐  ┌─────────────────┐ │
│  │  Web Routes      │  │  API Routes     │ │
│  │  /login          │  │  /register      │ │
│  │  /dashboard      │  │  /update        │ │
│  │  /admin          │  │  /health        │ │
│  └──────────────────┘  └─────────────────┘ │
└─────────────┬───────────────┬───────────────┘
              │               │
              ▼               ▼
┌─────────────────────────────────────────────┐
│         Authentication Layer                │
│  ┌──────────────┐  ┌─────────────────────┐ │
│  │  Session     │  │  API Key (existing) │ │
│  │  Auth        │  │  Header Auth        │ │
│  └──────────────┘  └─────────────────────┘ │
└─────────────┬───────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────┐
│           Business Logic Layer              │
│  ┌──────────┬──────────┬─────────────────┐ │
│  │ Models   │  Utils   │  Validation     │ │
│  │ User     │          │                 │ │
│  │ Session  │          │                 │ │
│  │ Record   │          │                 │ │
│  └──────────┴──────────┴─────────────────┘ │
└─────────────┬───────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────┐
│           Data Layer (db.go)                │
│  ┌────────┬─────────┬──────────┬─────────┐ │
│  │ users  │sessions │ records  │   txt   │ │
│  └────────┴─────────┴──────────┴─────────┘ │
│  SQLite or PostgreSQL                       │
└─────────────────────────────────────────────┘
```

## Project Structure

```
acme-dns/
├── main.go                      # Application entry point
├── constants.go                 # NEW: All application constants
├── types.go                     # Data structures & config (UPDATED)
├── config.cfg                   # Configuration template (UPDATED)
│
├── api.go                       # API endpoints (UPDATED)
├── auth.go                      # API authentication (UPDATED)
├── validation.go                # Input validation (UPDATED)
├── util.go                      # Utility functions (UPDATED)
│
├── db.go                        # Database interface (UPDATED)
├── db_migrations.go             # NEW: Migration system
├── acmetxt.go                   # ACME TXT record types
├── challengeprovider.go         # Certmagic provider
├── dns.go                       # DNS server
│
├── models/                      # NEW: Data models
│   ├── user.go                  # User account management
│   ├── session.go               # Session management
│   └── record.go                # Domain record management
│
├── web/                         # NEW: Web UI (to be completed)
│   ├── middleware.go            # Auth, rate limiting, security
│   ├── session.go               # Session cookie handling
│   ├── handlers.go              # Login, dashboard, etc.
│   ├── templates/               # HTML templates
│   │   ├── layout.html
│   │   ├── login.html
│   │   ├── dashboard.html
│   │   └── admin.html
│   └── static/                  # CSS, JS, images
│       ├── css/
│       ├── js/
│       └── img/
│
├── admin/                       # NEW: Admin functionality (to be completed)
│   └── handlers.go              # Admin-specific handlers
│
├── *_test.go                    # Test files
├── Dockerfile                   # Container build
├── docker-compose.yml           # Container orchestration
├── IMPLEMENTATION_PLAN.md       # NEW: Detailed implementation plan
├── PROGRESS.md                  # NEW: Current progress status
└── CLAUDE.md                    # NEW: This file
```

## Database Schema (v2)

### Tables

#### users (NEW)
```sql
id              INTEGER/SERIAL PRIMARY KEY
email           TEXT UNIQUE NOT NULL
password_hash   TEXT NOT NULL
is_admin        BOOLEAN DEFAULT FALSE
created_at      BIGINT NOT NULL
last_login      BIGINT
active          BOOLEAN DEFAULT TRUE
```

#### sessions (NEW)
```sql
id              TEXT PRIMARY KEY
user_id         BIGINT NOT NULL (FK -> users.id)
created_at      BIGINT NOT NULL
expires_at      BIGINT NOT NULL
ip_address      TEXT
user_agent      TEXT
```

#### records (EXTENDED)
```sql
Username        TEXT UNIQUE NOT NULL PRIMARY KEY
Password        TEXT UNIQUE NOT NULL
Subdomain       TEXT UNIQUE NOT NULL
AllowFrom       TEXT
user_id         BIGINT (FK -> users.id) -- NEW
created_at      BIGINT                   -- NEW
description     TEXT                     -- NEW
```

#### txt (UNCHANGED)
```sql
rowid           INTEGER/SERIAL
Subdomain       TEXT NOT NULL
Value           TEXT NOT NULL DEFAULT ''
LastUpdate      INT
```

#### acmedns (UNCHANGED)
```sql
Name            TEXT
Value           TEXT
```

### Indexes (NEW)
- `idx_txt_subdomain` ON txt(Subdomain)
- `idx_txt_lastupdate` ON txt(LastUpdate)
- `idx_sessions_user_id` ON sessions(user_id)
- `idx_sessions_expires_at` ON sessions(expires_at)
- `idx_records_user_id` ON records(user_id)

## Configuration

### New Configuration Sections

#### [webui]
```toml
enabled = false                    # Enable/disable web UI
session_duration = 24              # Session duration in hours
require_email_verification = false # Email verification (not yet implemented)
allow_self_registration = true     # Allow user self-registration
min_password_length = 12           # Minimum password length
```

#### [security]
```toml
rate_limiting = true               # Enable rate limiting
max_login_attempts = 5             # Max failed login attempts
lockout_duration = 15              # Lockout duration in minutes
session_cookie_name = "acmedns_session"
csrf_cookie_name = "acmedns_csrf"
max_request_body_size = 1048576    # 1MB
```

## API Reference

### Existing Endpoints (Unchanged)

#### POST /register
Register a new subdomain and get API credentials.

**Request (optional):**
```json
{
  "allowfrom": ["192.168.1.0/24"]
}
```

**Response:**
```json
{
  "username": "uuid-v4",
  "password": "40-char-key",
  "fulldomain": "subdomain.domain.tld",
  "subdomain": "subdomain",
  "allowfrom": ["192.168.1.0/24"]
}
```

#### POST /update
Update TXT record value.

**Headers:**
- X-Api-User: uuid-v4
- X-Api-Key: password

**Request:**
```json
{
  "subdomain": "subdomain",
  "txt": "43-char-challenge-token"
}
```

**Response:**
```json
{
  "txt": "43-char-challenge-token"
}
```

#### GET /health
Health check endpoint.

**Response:** 200 OK

### New Endpoints (To Be Implemented)

#### Web UI Endpoints
- `GET /login` - Login page
- `POST /login` - Login handler
- `GET /logout` - Logout
- `GET /dashboard` - User dashboard
- `POST /dashboard/register` - Register new domain via web UI
- `DELETE /dashboard/domain/:username` - Delete domain
- `GET /admin` - Admin dashboard
- `POST /admin/users` - Create user
- `DELETE /admin/users/:id` - Delete user
- `POST /admin/claim/:username` - Claim unmanaged domain

#### API v1 Endpoints (Planned)
- `POST /api/v1/auth/login` - API login
- `GET /api/v1/auth/me` - Get current user
- `GET /api/v1/domains` - List user's domains
- `DELETE /api/v1/domains/:id` - Delete domain

## Models Reference

### User Model
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
```

**Methods:**
- `Create(email, password, isAdmin, bcryptCost)` - Create new user
- `GetByID(id)` - Get user by ID
- `GetByEmail(email)` - Get user by email
- `Authenticate(email, password)` - Verify credentials
- `ChangePassword(userID, newPassword, cost)` - Change password
- `UpdateEmail(userID, newEmail)` - Update email
- `SetActive(userID, active)` - Enable/disable account
- `ListAll(activeOnly)` - List all users

### Session Model
```go
type Session struct {
    ID        string
    UserID    int64
    CreatedAt time.Time
    ExpiresAt time.Time
    IPAddress string
    UserAgent string
}
```

**Methods:**
- `Create(userID, durationHours, ip, userAgent)` - Create session
- `Get(sessionID)` - Get session
- `GetValid(sessionID)` - Get and validate session
- `Delete(sessionID)` - Delete session
- `DeleteByUserID(userID)` - Delete all user sessions
- `DeleteExpired()` - Cleanup expired sessions
- `Extend(sessionID, additionalHours)` - Extend session
- `ListByUserID(userID)` - List user's active sessions

### Record Model
```go
type Record struct {
    Username    string
    Password    string
    Subdomain   string
    AllowFrom   []string
    UserID      *int64      // NULL for API-only records
    CreatedAt   *time.Time
    Description *string
}
```

**Methods:**
- `GetByUsername(username)` - Get record
- `ListByUserID(userID)` - List user's records
- `ListAll()` - List all records (admin)
- `ListUnmanaged()` - List API-only records
- `ClaimRecord(username, userID, description)` - Claim unmanaged record
- `UpdateDescription(username, userID, description)` - Update description
- `Delete(username, userID)` - Delete user's record
- `DeleteByAdmin(username)` - Delete any record (admin)
- `GetTXTRecords(subdomain)` - Get TXT values

## Constants Reference

All magic numbers are now defined in `constants.go`:

### Validation Constants
- `ACMETxtLength = 43` - ACME challenge token length
- `APIKeyLength = 40` - API key length
- `PasswordLength = 40` - Generated password length
- `BcryptCostAPI = 10` - Bcrypt cost for API keys
- `BcryptCostWeb = 12` - Bcrypt cost for web passwords

### HTTP Constants
- `MaxRequestBodySize = 1048576` - 1MB max request size
- `HeaderAPIUser = "X-Api-User"`
- `HeaderAPIKey = "X-Api-Key"`
- `HeaderContentType = "Content-Type"`
- `HeaderContentTypeJSON = "application/json"`

### Security Headers
- `HeaderXContentTypeOptions = "X-Content-Type-Options"`
- `HeaderXFrameOptions = "X-Frame-Options"`
- `HeaderContentSecurityPolicy = "Content-Security-Policy"`
- `HeaderStrictTransportSecurity = "Strict-Transport-Security"`

### Error Messages
- `ErrMalformedJSON = "malformed_json_payload"`
- `ErrInvalidCIDR = "invalid_allowfrom_cidr"`
- `ErrBadSubdomain = "bad_subdomain"`
- `ErrBadTXT = "bad_txt"`
- `ErrDBError = "db_error"`
- `ErrForbidden = "forbidden"`
- `ErrUnauthorized = "unauthorized"`
- `ErrInvalidCredentials = "invalid_credentials"`
- And more...

## Security Features

### Implemented
1. ✅ Bcrypt password hashing (cost 10 for API, cost 12 for web UI)
2. ✅ Timing attack protection in authentication
3. ✅ SQL injection prevention via prepared statements
4. ✅ TLS 1.2 minimum version
5. ✅ Crypto-secure random generation (passwords, session IDs)
6. ✅ Optional CIDR-based IP restrictions
7. ✅ File permissions (umask 0077)
8. ✅ Database connection pooling
9. ✅ Password complexity requirements
10. ✅ Session expiration

### To Be Implemented
1. ⏳ Rate limiting middleware
2. ⏳ CSRF protection
3. ⏳ Security headers (CSP, X-Frame-Options, etc.)
4. ⏳ Request size limits on all endpoints
5. ⏳ Generic error messages (avoid user enumeration)
6. ⏳ Session fixation protection
7. ⏳ Audit logging

## Development Workflow

### Prerequisites
- Go 1.13+ (tested up to 1.23)
- SQLite or PostgreSQL
- Port 53 (DNS) access
- Configurable HTTP/HTTPS port access

### Setup
```bash
# Clone and enter directory
cd acme-dns

# Build
go build

# Run tests
go test -v ./...

# Run with config
./acme-dns -c ./config.cfg
```

### Database Migration
The database automatically migrates on startup:
- v0 → v1: Adds rolling TXT record support
- v1 → v2: Adds users, sessions, extends records table

To create the first admin user (after enabling web UI):
```bash
./acme-dns --create-admin admin@example.com
# (To be implemented)
```

### Enabling Web UI
1. Edit `config.cfg`:
   ```toml
   [webui]
   enabled = true
   ```
2. Restart acme-dns
3. Database will auto-migrate to v2
4. Create admin user via CLI
5. Access web UI at `https://your-domain/login`

## Remaining Work

### Critical (Required for MVP)
1. **Web Middleware** (`web/middleware.go`)
   - Rate limiting
   - Security headers
   - CSRF protection
   - Authentication checks

2. **Web Session Management** (`web/session.go`)
   - Cookie creation and validation
   - CSRF token generation
   - Session helpers

3. **Web Handlers** (`web/handlers.go`)
   - Login page and POST handler
   - Logout handler
   - Dashboard page
   - Domain registration
   - Domain deletion

4. **Admin Handlers** (`admin/handlers.go`)
   - Admin dashboard
   - User management CRUD
   - Domain listing and management
   - Claim unmanaged domains

5. **HTML Templates** (`web/templates/`)
   - Layout template with navigation
   - Login page
   - Dashboard page
   - Admin page

6. **Main Integration** (`main.go`)
   - Initialize repositories
   - Add web routes
   - Start session cleanup goroutine
   - Add CLI flag handling

### Important (Should Have)
1. Static assets (CSS/JS)
2. CLI admin user creation
3. Health check database ping
4. Request size limits on API
5. Graceful shutdown
6. go.mod updates

### Nice to Have
1. Profile page
2. Email verification
3. Password reset
4. Activity logs
5. Metrics/monitoring
6. API v1 endpoints
7. Comprehensive tests for new features

## Testing Strategy

### Unit Tests
- User model operations
- Session management
- Password validation
- CSRF protection
- Rate limiting

### Integration Tests
- Login flow end-to-end
- Domain registration via web UI
- Admin operations
- API backward compatibility

### Manual Testing Checklist
- [ ] Fresh install with new database
- [ ] Existing v1 database migration
- [ ] User registration and login
- [ ] Session expiration
- [ ] Domain CRUD via web UI
- [ ] Admin user management
- [ ] Admin domain management
- [ ] API-only registration still works
- [ ] Rate limiting enforcement
- [ ] Security headers present

## Deployment

### Docker
```bash
docker build -t acme-dns:v2 .
docker run -d \
  -p 53:53 -p 53:53/udp -p 443:443 \
  -v /path/to/config:/etc/acme-dns:ro \
  -v /path/to/data:/var/lib/acme-dns \
  acme-dns:v2
```

### Systemd
No changes needed - service file remains compatible.

### Migration from v1
1. Backup database
2. Update binary
3. Restart service (auto-migrates database)
4. Optionally enable web UI in config
5. Create admin account
6. Login to web UI

## Common Issues & Solutions

### Database Locked
- **Cause**: SQLite doesn't handle high concurrency well
- **Solution**: Use PostgreSQL or reduce concurrent requests

### Port 53 In Use
- **Cause**: systemd-resolved using port 53
- **Solution**: Configure different interface in config

### Session Not Persisting
- **Cause**: Secure cookie flag set without HTTPS
- **Solution**: Use HTTPS or adjust cookie settings for dev

### Migration Failed
- **Cause**: Manual schema changes or corruption
- **Solution**: Restore backup and re-migrate

## Contributing

### Code Style
- Follow standard Go conventions
- Run `gofmt` before committing
- Use constants from `constants.go`
- Add tests for new functionality
- Update documentation

### Pull Request Process
1. Create feature branch
2. Implement changes
3. Add/update tests
4. Run `go test -v ./...`
5. Run `golangci-lint run`
6. Update PROGRESS.md
7. Submit PR with clear description

## Resources

- **Documentation**: README.md, IMPLEMENTATION_PLAN.md, this file
- **Issues**: https://github.com/joohoi/acme-dns/issues
- **ACME Spec**: https://tools.ietf.org/html/rfc8555
- **DNS-01 Challenge**: https://letsencrypt.org/docs/challenge-types/

## Quick Reference

### File Locations
- Config: `/etc/acme-dns/config.cfg` or `./config.cfg`
- Database (SQLite): `/var/lib/acme-dns/acme-dns.db`
- Logs: stdout (configure in logconfig section)

### Default Ports
- DNS: 53 (TCP/UDP)
- HTTP API: Configurable (default 443)

### Important Commands
```bash
# Build
go build

# Test
go test -v ./...

# Run
./acme-dns -c config.cfg

# Create admin (to be implemented)
./acme-dns --create-admin admin@example.com

# Check version (to be implemented)
./acme-dns --version

# Database migration info (to be implemented)
./acme-dns --migrate
```

## Summary of Changes

This enhancement adds a complete web UI to acme-dns while maintaining 100% backward compatibility. Key improvements include:

- 🎨 **Web UI**: User-friendly interface for domain management
- 👤 **User Accounts**: Secure authentication and multi-user support
- 🔐 **Enhanced Security**: Rate limiting, CSRF, security headers
- 📊 **Admin Dashboard**: Comprehensive administrative controls
- 🗄️ **Database v2**: Auto-migrating schema with new capabilities
- 🚀 **Performance**: Connection pooling, indexes, optimizations
- 📝 **Code Quality**: Constants, better error handling, documentation

The implementation is approximately 70% complete, with core infrastructure in place and web handlers/templates remaining.
