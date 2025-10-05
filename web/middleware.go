package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/joohoi/acme-dns/models"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// ContextKey is a type for context keys
type ContextKey string

const (
	// UserIDKey is the context key for user ID
	UserIDKey ContextKey = "user_id"

	// IsAdminKey is the context key for admin status
	IsAdminKey ContextKey = "is_admin"

	// CSRFTokenKey is the context key for CSRF token
	CSRFTokenKey ContextKey = "csrf_token"
)

// RateLimiter holds rate limiters for IP addresses
type RateLimiter struct {
	visitors map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int, burst int) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerMinute),
		burst:    burst,
	}
}

// GetLimiter returns the rate limiter for an IP address
func (rl *RateLimiter) GetLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.visitors[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = limiter
	}

	return limiter
}

// Cleanup removes old entries from the rate limiter map
func (rl *RateLimiter) Cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			rl.mu.Lock()
			// In a production system, you'd track last access time
			// For now, we just keep the map from growing unbounded by clearing it periodically
			if len(rl.visitors) > 10000 {
				rl.visitors = make(map[string]*rate.Limiter)
			}
			rl.mu.Unlock()
		}
	}()
}

// getIPAddress extracts the IP address from the request
func getIPAddress(r *http.Request) string {
	// Try X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP if there are multiple
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Try X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimitMiddleware creates a rate limiting middleware
func RateLimitMiddleware(rl *RateLimiter, enabled bool) func(httprouter.Handle) httprouter.Handle {
	return func(next httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			if !enabled {
				next(w, r, ps)
				return
			}

			ip := getIPAddress(r)
			limiter := rl.GetLimiter(ip)

			if !limiter.Allow() {
				log.WithFields(log.Fields{"ip": ip, "path": r.URL.Path}).Warn("Rate limit exceeded")
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next(w, r, ps)
		}
	}
}

// SecurityHeadersMiddleware adds security headers to responses
func SecurityHeadersMiddleware(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Content Security Policy
		csp := "default-src 'self'; " +
			"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
			"script-src 'self' https://cdn.jsdelivr.net; " +
			"font-src 'self' https://cdn.jsdelivr.net; " +
			"img-src 'self' data:;"
		w.Header().Set("Content-Security-Policy", csp)

		// HSTS - force HTTPS (only if request is HTTPS)
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// XSS Protection (legacy, but doesn't hurt)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions Policy (formerly Feature Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next(w, r, ps)
	}
}

// RequestSizeLimitMiddleware limits the size of request bodies
func RequestSizeLimitMiddleware(maxBytes int64) func(httprouter.Handle) httprouter.Handle {
	return func(next httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			// Limit request body size
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			next(w, r, ps)
		}
	}
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		start := time.Now()

		// Create a response wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next(wrapped, r, ps)

		duration := time.Since(start)
		log.WithFields(log.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      wrapped.statusCode,
			"duration_ms": duration.Milliseconds(),
			"ip":          getIPAddress(r),
		}).Info("HTTP request")
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// ChainMiddleware chains multiple middleware functions
func ChainMiddleware(h httprouter.Handle, middlewares ...func(httprouter.Handle) httprouter.Handle) httprouter.Handle {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// RequireAuth middleware ensures the user is authenticated
func RequireAuth(sm *SessionManager) func(httprouter.Handle) httprouter.Handle {
	return func(next httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			session, err := sm.GetSession(r)
			if err != nil || session == nil {
				// Not authenticated, redirect to login
				log.WithFields(log.Fields{"path": r.URL.Path, "error": err}).Debug("Authentication required")
				http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
				return
			}

			// Add user info to context
			ctx := context.WithValue(r.Context(), UserIDKey, session.UserID)

			// Check if user is admin (would need to query database)
			// For now, we'll add it in the handler

			next(w, r.WithContext(ctx), ps)
		}
	}
}

// RequireAdmin middleware ensures the user is an admin
func RequireAdmin(sm *SessionManager, userRepo interface {
	GetByID(int64) (*models.User, error)
}) func(httprouter.Handle) httprouter.Handle {
	return func(next httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			session, err := sm.GetSession(r)
			if err != nil || session == nil {
				log.WithFields(log.Fields{"path": r.URL.Path}).Debug("Admin access denied - not authenticated")
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			// Get user from database to check admin status
			user, err := userRepo.GetByID(session.UserID)
			if err != nil || user == nil || !user.IsAdmin {
				log.WithFields(log.Fields{"path": r.URL.Path, "user_id": session.UserID}).Warn("Admin access denied - not admin")
				http.Error(w, "Forbidden - Admin access required", http.StatusForbidden)
				return
			}

			// Add user info to context
			ctx := context.WithValue(r.Context(), UserIDKey, session.UserID)
			ctx = context.WithValue(ctx, IsAdminKey, true)

			next(w, r.WithContext(ctx), ps)
		}
	}
}

// CSRFMiddleware validates CSRF tokens for state-changing requests
func CSRFMiddleware(sm *SessionManager) func(httprouter.Handle) httprouter.Handle {
	return func(next httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			// Skip CSRF check for GET, HEAD, OPTIONS
			if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
				next(w, r, ps)
				return
			}

			// Get CSRF token from session
			session, err := sm.GetSession(r)
			if err != nil || session == nil {
				http.Error(w, "Invalid session", http.StatusForbidden)
				return
			}

			csrfToken := sm.GetCSRFToken(session.ID)
			if csrfToken == "" {
				http.Error(w, "CSRF token not found", http.StatusForbidden)
				return
			}

			// Check CSRF token from form or header
			formToken := r.FormValue("csrf_token")
			if formToken == "" {
				formToken = r.Header.Get("X-CSRF-Token")
			}

			if formToken != csrfToken {
				log.WithFields(log.Fields{
					"path":     r.URL.Path,
					"expected": csrfToken[:10] + "...",
					"got":      formToken[:10] + "...",
				}).Warn("CSRF token mismatch")
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}

			next(w, r, ps)
		}
	}
}

// RecoverMiddleware recovers from panics and logs them
func RecoverMiddleware(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		defer func() {
			if err := recover(); err != nil {
				log.WithFields(log.Fields{
					"error": err,
					"path":  r.URL.Path,
				}).Error("Panic recovered")

				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		next(w, r, ps)
	}
}

// CORSMiddleware handles CORS for API endpoints
func CORSMiddleware(allowedOrigins []string) func(httprouter.Handle) httprouter.Handle {
	return func(next httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Api-User, X-Api-Key")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// Handle preflight
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next(w, r, ps)
		}
	}
}

// GetUserIDFromContext extracts user ID from context
func GetUserIDFromContext(ctx context.Context) (int64, error) {
	userID, ok := ctx.Value(UserIDKey).(int64)
	if !ok {
		return 0, fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}

// IsAdminFromContext checks if user is admin from context
func IsAdminFromContext(ctx context.Context) bool {
	isAdmin, ok := ctx.Value(IsAdminKey).(bool)
	return ok && isAdmin
}
