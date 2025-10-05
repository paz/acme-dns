package main

// API constants
const (
	// ACMETxtLength is the expected length of ACME challenge TXT records
	// This is the current Let's Encrypt auth key size
	ACMETxtLength = 43

	// APIKeyLength is the expected length of API keys
	APIKeyLength = 40

	// PasswordLength is the length of generated passwords for API access
	PasswordLength = 40

	// BcryptCostAPI is the bcrypt cost for API key hashing
	BcryptCostAPI = 10

	// BcryptCostWeb is the bcrypt cost for web UI password hashing (higher security)
	BcryptCostWeb = 12

	// MaxRequestBodySize is the maximum size of HTTP request bodies (1MB)
	MaxRequestBodySize = 1024 * 1024

	// DefaultSessionDuration is the default session duration in hours
	DefaultSessionDuration = 24

	// DefaultRateLimit is the default rate limit for API endpoints
	DefaultRateLimit = 10

	// SessionIDLength is the length of session IDs
	SessionIDLength = 64
)

// Database version constants
const (
	// CurrentDBVersion is the current database schema version
	CurrentDBVersion = 2

	// PreviousDBVersion is the previous database schema version
	PreviousDBVersion = 1
)

// HTTP header names
const (
	// HeaderAPIUser is the header name for API username
	HeaderAPIUser = "X-Api-User"

	// HeaderAPIKey is the header name for API key
	HeaderAPIKey = "X-Api-Key"

	// HeaderContentType is the standard Content-Type header
	HeaderContentType = "Content-Type"

	// HeaderContentTypeJSON is the JSON content type
	HeaderContentTypeJSON = "application/json"
)

// Security headers
const (
	// HeaderXContentTypeOptions prevents MIME type sniffing
	HeaderXContentTypeOptions = "X-Content-Type-Options"

	// HeaderXFrameOptions prevents clickjacking
	HeaderXFrameOptions = "X-Frame-Options"

	// HeaderContentSecurityPolicy defines CSP policy
	HeaderContentSecurityPolicy = "Content-Security-Policy"

	// HeaderStrictTransportSecurity enforces HTTPS
	HeaderStrictTransportSecurity = "Strict-Transport-Security"

	// HeaderXXSSProtection enables XSS filter
	HeaderXXSSProtection = "X-XSS-Protection"
)

// Security header values
const (
	// ValueNoSniff prevents MIME type sniffing
	ValueNoSniff = "nosniff"

	// ValueDeny denies framing
	ValueDeny = "DENY"

	// ValueCSPDefault is a basic CSP policy
	ValueCSPDefault = "default-src 'self'; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; script-src 'self' https://cdn.jsdelivr.net; font-src 'self' https://cdn.jsdelivr.net"

	// ValueHSTSDefault is HSTS policy for 1 year
	ValueHSTSDefault = "max-age=31536000; includeSubDomains"

	// ValueXSSProtection enables XSS filter in legacy browsers
	ValueXSSProtection = "1; mode=block"
)

// Error messages
const (
	// ErrMalformedJSON indicates malformed JSON payload
	ErrMalformedJSON = "malformed_json_payload"

	// ErrInvalidCIDR indicates invalid CIDR mask in allowfrom
	ErrInvalidCIDR = "invalid_allowfrom_cidr"

	// ErrBadSubdomain indicates bad subdomain format
	ErrBadSubdomain = "bad_subdomain"

	// ErrBadTXT indicates bad TXT record format
	ErrBadTXT = "bad_txt"

	// ErrDBError indicates database error
	ErrDBError = "db_error"

	// ErrForbidden indicates forbidden access
	ErrForbidden = "forbidden"

	// ErrUnauthorized indicates authentication failure
	ErrUnauthorized = "unauthorized"

	// ErrInvalidCredentials indicates invalid credentials (generic message for security)
	ErrInvalidCredentials = "invalid_credentials"

	// ErrNotFound indicates resource not found
	ErrNotFound = "not_found"

	// ErrRateLimitExceeded indicates rate limit exceeded
	ErrRateLimitExceeded = "rate_limit_exceeded"

	// ErrInvalidEmail indicates invalid email format
	ErrInvalidEmail = "invalid_email"

	// ErrWeakPassword indicates password doesn't meet requirements
	ErrWeakPassword = "weak_password"

	// ErrUserExists indicates user already exists
	ErrUserExists = "user_already_exists"

	// ErrSessionExpired indicates session has expired
	ErrSessionExpired = "session_expired"

	// ErrCSRFInvalid indicates invalid CSRF token
	ErrCSRFInvalid = "invalid_csrf_token"
)

// Default configuration values
const (
	// DefaultACMECacheDir is the default directory for ACME certificates
	DefaultACMECacheDir = "api-certs"

	// DefaultMinPasswordLength is the minimum password length for web UI
	DefaultMinPasswordLength = 12

	// DefaultMaxLoginAttempts is the default max login attempts before lockout
	DefaultMaxLoginAttempts = 5

	// DefaultLockoutDuration is the default lockout duration in minutes
	DefaultLockoutDuration = 15

	// DefaultSessionCookieName is the default session cookie name
	DefaultSessionCookieName = "acmedns_session"

	// DefaultCSRFCookieName is the default CSRF cookie name
	DefaultCSRFCookieName = "acmedns_csrf"
)

// Database connection pool defaults
const (
	// DefaultMaxOpenConns is the default maximum number of open database connections
	DefaultMaxOpenConns = 25

	// DefaultMaxIdleConns is the default maximum number of idle database connections
	DefaultMaxIdleConns = 5

	// DefaultConnMaxLifetimeMinutes is the default maximum lifetime of a connection in minutes
	DefaultConnMaxLifetimeMinutes = 5
)
