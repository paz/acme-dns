package web

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/joohoi/acme-dns/models"
	log "github.com/sirupsen/logrus"
)

// SessionManager handles session creation, validation, and cookie management
type SessionManager struct {
	sessionRepo  SessionRepository
	cookieName   string
	csrfTokens   map[string]string // sessionID -> csrfToken
	csrfMutex    sync.RWMutex
	secureCookie bool
}

// SessionRepository interface for session storage
type SessionRepository interface {
	Create(userID int64, durationHours int, ipAddress, userAgent string) (*models.Session, error)
	GetValid(sessionID string) (*models.Session, error)
	Delete(sessionID string) error
	Extend(sessionID string, additionalHours int) error
}

// NewSessionManager creates a new session manager
func NewSessionManager(repo SessionRepository, cookieName string, secureCookie bool) *SessionManager {
	return &SessionManager{
		sessionRepo:  repo,
		cookieName:   cookieName,
		csrfTokens:   make(map[string]string),
		secureCookie: secureCookie,
	}
}

// CreateSession creates a new session and sets the cookie
func (sm *SessionManager) CreateSession(w http.ResponseWriter, r *http.Request, userID int64, durationHours int) (*models.Session, error) {
	// Get client info
	ipAddress := getIPAddress(r)
	userAgent := r.UserAgent()

	// Create session in database
	session, err := sm.sessionRepo.Create(userID, durationHours, ipAddress, userAgent)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Generate CSRF token
	csrfToken, err := generateCSRFToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	// Store CSRF token
	sm.csrfMutex.Lock()
	sm.csrfTokens[session.ID] = csrfToken
	sm.csrfMutex.Unlock()

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   sm.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	log.WithFields(log.Fields{
		"session_id": session.ID,
		"user_id":    userID,
	}).Debug("Session created")

	return session, nil
}

// GetSession retrieves the session from the cookie
func (sm *SessionManager) GetSession(r *http.Request) (*models.Session, error) {
	cookie, err := r.Cookie(sm.cookieName)
	if err != nil {
		return nil, fmt.Errorf("session cookie not found: %w", err)
	}

	session, err := sm.sessionRepo.GetValid(cookie.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}

	return session, nil
}

// DestroySession destroys the session and clears the cookie
func (sm *SessionManager) DestroySession(w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(sm.cookieName)
	if err != nil {
		// No cookie to destroy
		return nil
	}

	// Delete from database
	err = sm.sessionRepo.Delete(cookie.Value)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "session_id": cookie.Value}).Warn("Failed to delete session")
	}

	// Remove CSRF token
	sm.csrfMutex.Lock()
	delete(sm.csrfTokens, cookie.Value)
	sm.csrfMutex.Unlock()

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   sm.secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	return nil
}

// ExtendSession extends the session expiration time
func (sm *SessionManager) ExtendSession(w http.ResponseWriter, r *http.Request, additionalHours int) error {
	session, err := sm.GetSession(r)
	if err != nil {
		return err
	}

	err = sm.sessionRepo.Extend(session.ID, additionalHours)
	if err != nil {
		return fmt.Errorf("failed to extend session: %w", err)
	}

	// Update cookie expiration
	newExpiry := time.Now().Add(time.Duration(additionalHours) * time.Hour)
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    session.ID,
		Path:     "/",
		Expires:  newExpiry,
		HttpOnly: true,
		Secure:   sm.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	return nil
}

// GetCSRFToken returns the CSRF token for a session
func (sm *SessionManager) GetCSRFToken(sessionID string) string {
	sm.csrfMutex.RLock()
	defer sm.csrfMutex.RUnlock()

	return sm.csrfTokens[sessionID]
}

// generateCSRFToken generates a random CSRF token
func generateCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// FlashMessage represents a temporary message to display to the user
type FlashMessage struct {
	Type    string // "success", "error", "warning", "info"
	Message string
}

// FlashStore stores flash messages in memory (in production, use session storage)
type FlashStore struct {
	messages map[string][]FlashMessage
	mu       sync.RWMutex
}

// NewFlashStore creates a new flash message store
func NewFlashStore() *FlashStore {
	return &FlashStore{
		messages: make(map[string][]FlashMessage),
	}
}

// Add adds a flash message for a session
func (fs *FlashStore) Add(sessionID, msgType, message string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	fs.messages[sessionID] = append(fs.messages[sessionID], FlashMessage{
		Type:    msgType,
		Message: message,
	})
}

// Get retrieves and clears flash messages for a session
func (fs *FlashStore) Get(sessionID string) []FlashMessage {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	messages := fs.messages[sessionID]
	delete(fs.messages, sessionID)
	return messages
}

// AddFlash adds a flash message for the current session
func (sm *SessionManager) AddFlash(r *http.Request, fs *FlashStore, msgType, message string) {
	session, err := sm.GetSession(r)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warn("Failed to add flash message - no session")
		return
	}

	fs.Add(session.ID, msgType, message)
}

// GetFlashes retrieves flash messages for the current session
func (sm *SessionManager) GetFlashes(r *http.Request, fs *FlashStore) []FlashMessage {
	session, err := sm.GetSession(r)
	if err != nil {
		return nil
	}

	return fs.Get(session.ID)
}

// TemplateData is a helper struct for template rendering
type TemplateData struct {
	Title       string
	User        interface{}
	IsAdmin     bool
	CSRFToken   string
	Flashes     []FlashMessage
	Data        map[string]interface{}
	CurrentPath string
}

// NewTemplateData creates a new template data struct with common fields populated
func (sm *SessionManager) NewTemplateData(r *http.Request, fs *FlashStore, title string) *TemplateData {
	td := &TemplateData{
		Title:       title,
		Data:        make(map[string]interface{}),
		Flashes:     sm.GetFlashes(r, fs),
		CurrentPath: r.URL.Path,
	}

	// Try to get session info
	session, err := sm.GetSession(r)
	if err == nil {
		td.CSRFToken = sm.GetCSRFToken(session.ID)
		// User info would be populated by the handler
	}

	return td
}
