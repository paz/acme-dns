package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/joohoi/acme-dns/email"
	"github.com/joohoi/acme-dns/models"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// Handlers holds all dependencies for web handlers
type Handlers struct {
	sessionManager    *SessionManager
	flashStore        *FlashStore
	userRepo          UserRepository
	recordRepo        RecordRepository
	sessionRepo       SessionRepositoryInterface
	passwordResetRepo *models.PasswordResetRepository
	mailer            *email.Mailer
	templates         *template.Template
	config            WebConfig
	domain            string
	baseURL           string
}

// WebConfig holds web UI configuration
type WebConfig struct {
	AllowSelfRegistration bool
	MinPasswordLength     int
}

// UserRepository interface for user operations
type UserRepository interface {
	Authenticate(email, password string) (*models.User, error)
	GetByID(id int64) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	Create(email, password string, isAdmin bool, bcryptCost int) (*models.User, error)
	ChangePassword(userID int64, newPassword string, bcryptCost int) error
}

// SessionRepositoryInterface for session operations (profile page needs this)
type SessionRepositoryInterface interface {
	Get(sessionID string) (*models.Session, error)
	Delete(sessionID string) error
	ListByUserID(userID int64) ([]*models.Session, error)
}

// RecordRepository interface for record operations
type RecordRepository interface {
	ListByUserID(userID int64) ([]*models.Record, error)
	GetByUsername(username string) (*models.Record, error)
	Delete(username string, userID int64) error
	UpdateDescription(username string, userID int64, description string) error
}

// NewHandlers creates a new handlers instance
func NewHandlers(
	sm *SessionManager,
	fs *FlashStore,
	userRepo UserRepository,
	recordRepo RecordRepository,
	sessionRepo SessionRepositoryInterface,
	passwordResetRepo *models.PasswordResetRepository,
	mailer *email.Mailer,
	templatesDir string, // Kept for backward compatibility but not used
	config WebConfig,
	domain string,
	baseURL string,
) (*Handlers, error) {
	// Load templates from embedded filesystem
	templates, err := GetTemplates()
	if err != nil {
		return nil, err
	}

	return &Handlers{
		sessionManager:    sm,
		flashStore:        fs,
		userRepo:          userRepo,
		recordRepo:        recordRepo,
		sessionRepo:       sessionRepo,
		passwordResetRepo: passwordResetRepo,
		mailer:            mailer,
		templates:         templates,
		config:            config,
		domain:            domain,
		baseURL:           baseURL,
	}, nil
}

// render executes a template by name
func (h *Handlers) render(w http.ResponseWriter, templateName string, data *TemplateData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Map template names to their content block names
	contentBlockMap := map[string]string{
		"login.html":                    "login-content",
		"dashboard.html":                "dashboard-content",
		"profile.html":                  "profile-content",
		"register.html":                 "register-content",
		"admin.html":                    "admin-content",
		"password_reset_request.html":   "password-reset-request-content",
		"password_reset.html":           "password-reset-content",
	}

	// Get the content block name for this template
	contentBlock, ok := contentBlockMap[templateName]
	if !ok {
		return h.templates.ExecuteTemplate(w, templateName, data)
	}

	// Clone the base template and add the specific content block
	tmpl, err := h.templates.Clone()
	if err != nil {
		return err
	}

	// Add the content block as "content" so the base template can find it
	tmpl, err = tmpl.AddParseTree("content", h.templates.Lookup(contentBlock).Tree)
	if err != nil {
		return err
	}

	// Execute the base template which will now use the correct content block
	return tmpl.ExecuteTemplate(w, "base", data)
}

// RootHandler redirects root to login or dashboard
func (h *Handlers) RootHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Check if already logged in
	if _, err := h.sessionManager.GetSession(r); err == nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// LoginPage displays the login page
func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Check if already logged in
	if _, err := h.sessionManager.GetSession(r); err == nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	data := h.sessionManager.NewTemplateData(r, h.flashStore, "Login")
	data.Data["AllowRegistration"] = h.config.AllowSelfRegistration

	// Get redirect parameter if present
	redirect := r.URL.Query().Get("redirect")
	if redirect != "" {
		data.Data["Redirect"] = redirect
	}

	// Render login page
	if err := h.render(w, "login.html", data); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to render login template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// LoginPost handles login form submission
func (h *Handlers) LoginPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	redirect := r.FormValue("redirect")

	// Authenticate user
	user, err := h.userRepo.Authenticate(email, password)
	if err != nil {
		log.WithFields(log.Fields{"email": email, "error": err}).Warn("Login failed")

		// Add flash message (we don't have session yet, so redirect with error)
		http.Redirect(w, r, "/login?error=invalid_credentials", http.StatusSeeOther)
		return
	}

	// Create session
	_, err = h.sessionManager.CreateSession(w, r, user.ID, 24) // 24 hours
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": user.ID}).Error("Failed to create session")
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{"user_id": user.ID, "email": email}).Info("User logged in")

	// Redirect to dashboard or requested page (with safe redirect validation)
	redirectURL := "/dashboard" // Default safe redirect
	if redirect != "" && isValidLocalRedirect(redirect) {
		redirectURL = redirect
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// isValidLocalRedirect checks if a redirect URL is safe (whitelist approach)
func isValidLocalRedirect(redirect string) bool {
	// Decode URL-encoded characters first to prevent bypasses
	decoded, err := url.QueryUnescape(redirect)
	if err != nil {
		return false
	}

	// Must start with / but not //
	if !strings.HasPrefix(decoded, "/") || strings.HasPrefix(decoded, "//") {
		return false
	}

	// Must not contain protocol
	if strings.Contains(decoded, "://") || strings.Contains(decoded, ":\\") {
		return false
	}

	// Must not be just /
	if decoded == "/" {
		return false
	}

	// Must not contain backslashes (Windows path traversal)
	if strings.Contains(decoded, "\\") {
		return false
	}

	// Whitelist approach: only allow specific paths
	allowedPaths := []string{"/dashboard", "/admin", "/profile"}
	for _, allowed := range allowedPaths {
		if strings.HasPrefix(decoded, allowed) {
			return true
		}
	}

	return false
}

// Logout handles user logout
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err == nil {
		log.WithFields(log.Fields{"user_id": session.UserID}).Info("User logged out")
	}

	if err := h.sessionManager.DestroySession(w, r); err != nil {
		log.WithFields(log.Fields{"error": err}).Warn("Error destroying session")
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// Dashboard displays the user's dashboard
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get user info
	user, err := h.userRepo.GetByID(session.UserID)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": session.UserID}).Error("Failed to get user")
		http.Error(w, "Failed to load user", http.StatusInternalServerError)
		return
	}

	// Get user's records
	records, err := h.recordRepo.ListByUserID(session.UserID)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": session.UserID}).Error("Failed to list records")
		http.Error(w, "Failed to load records", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := h.sessionManager.NewTemplateData(r, h.flashStore, "Dashboard")
	data.User = user
	data.IsAdmin = user.IsAdmin
	data.Data["Domains"] = records
	data.Data["Domain"] = h.domain

	if err := h.render(w, "dashboard.html", data); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to render dashboard template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// RegisterDomain handles domain registration via web UI
func (h *Handlers) RegisterDomain(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	description := r.FormValue("description")
	allowFromJSON := r.FormValue("allowfrom")

	// Parse allowfrom if provided
	var allowFrom []string
	if allowFromJSON != "" {
		if err := json.Unmarshal([]byte(allowFromJSON), &allowFrom); err != nil {
			http.Error(w, "Invalid allowfrom format", http.StatusBadRequest)
			return
		}
	}

	// Call the existing API registration logic
	// This would need to be refactored to be accessible from here
	// For now, return a TODO response

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Domain registration would happen here (integration with existing API logic needed)",
	}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}

	log.WithFields(log.Fields{
		"user_id":     session.UserID,
		"description": description,
	}).Info("Domain registered via web UI")
}

// DeleteDomain handles domain deletion
func (h *Handlers) DeleteDomain(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username := ps.ByName("username")

	err = h.recordRepo.Delete(username, session.UserID)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "username": username}).Error("Failed to delete domain")
		http.Error(w, "Failed to delete domain", http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{"user_id": session.UserID, "username": username}).Info("Domain deleted")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// UpdateDomainDescription handles updating a domain's description
func (h *Handlers) UpdateDomainDescription(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username := ps.ByName("username")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	description := r.FormValue("description")

	err = h.recordRepo.UpdateDescription(username, session.UserID, description)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "username": username}).Error("Failed to update description")
		http.Error(w, "Failed to update description", http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{"user_id": session.UserID, "username": username}).Info("Domain description updated")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// ViewDomainCredentials returns the credentials for a domain
func (h *Handlers) ViewDomainCredentials(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	username := ps.ByName("username")

	record, err := h.recordRepo.GetByUsername(username)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	// Verify ownership - critical security check
	if record.UserID == nil || *record.UserID != session.UserID {
		log.WithFields(log.Fields{
			"user_id":  session.UserID,
			"username": username,
		}).Warn("Unauthorized access attempt to domain credentials")
		http.Error(w, "Forbidden - you do not own this domain", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"username":    record.Username,
		"password":    record.Password,
		"subdomain":   record.Subdomain,
		"fulldomain":  record.Subdomain + "." + h.domain,
		"allowfrom":   record.AllowFrom,
		"description": record.Description,
	}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}

	log.WithFields(log.Fields{"user_id": session.UserID, "username": username}).Debug("Domain credentials viewed")
}

// Profile displays the user's profile page
func (h *Handlers) Profile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	user, err := h.userRepo.GetByID(session.UserID)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": session.UserID}).Error("Failed to get user")
		http.Error(w, "Failed to load user", http.StatusInternalServerError)
		return
	}

	data := h.sessionManager.NewTemplateData(r, h.flashStore, "Profile")
	data.User = user
	data.IsAdmin = user.IsAdmin

	// Would render a profile template (not yet created)
	if _, err := w.Write([]byte("Profile page - to be implemented with template")); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to write response")
	}
}

// RegisterPage displays the registration page (if self-registration is enabled)
func (h *Handlers) RegisterPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !h.config.AllowSelfRegistration {
		http.Error(w, "Registration is disabled", http.StatusForbidden)
		return
	}

	// Check if already logged in
	if _, err := h.sessionManager.GetSession(r); err == nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	data := h.sessionManager.NewTemplateData(r, h.flashStore, "Register")
	data.Data["MinPasswordLength"] = h.config.MinPasswordLength

	// Get error from query param if any
	if errorType := r.URL.Query().Get("error"); errorType != "" {
		data.Data["error"] = errorType
	}

	if err := h.render(w, "register.html", data); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to render register template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// RegisterPost handles user registration (if self-registration is enabled)
func (h *Handlers) RegisterPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if !h.config.AllowSelfRegistration {
		http.Error(w, "Registration is disabled", http.StatusForbidden)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	// Validate passwords match
	if password != confirmPassword {
		http.Redirect(w, r, "/register?error=passwords_dont_match", http.StatusSeeOther)
		return
	}

	// Create user (not as admin)
	user, err := h.userRepo.Create(email, password, false, 12) // bcrypt cost 12
	if err != nil {
		log.WithFields(log.Fields{"error": err, "email": email}).Warn("Registration failed")
		http.Redirect(w, r, "/register?error=registration_failed", http.StatusSeeOther)
		return
	}

	log.WithFields(log.Fields{"user_id": user.ID, "email": email}).Info("User registered")

	// Auto-login after registration
	_, err = h.sessionManager.CreateSession(w, r, user.ID, 24)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to create session after registration")
		http.Redirect(w, r, "/login?success=registered", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// ProfilePage displays the user profile page
func (h *Handlers) ProfilePage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get user info
	user, err := h.userRepo.GetByID(session.UserID)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": session.UserID}).Error("Failed to get user")
		http.Error(w, "Failed to load user", http.StatusInternalServerError)
		return
	}

	// Get user's active sessions
	sessions, err := h.sessionRepo.ListByUserID(session.UserID)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": session.UserID}).Error("Failed to list sessions")
		// Continue without sessions
		sessions = []*models.Session{}
	}

	// Prepare template data
	data := h.sessionManager.NewTemplateData(r, h.flashStore, "Profile")
	data.User = user
	data.IsAdmin = user.IsAdmin
	data.Data["Sessions"] = sessions
	data.Data["CurrentSessionID"] = session.ID

	if err := h.render(w, "profile.html", data); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to render profile template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ChangePassword handles password change requests
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Validate passwords match
	if newPassword != confirmPassword {
		h.sessionManager.AddFlash(r, h.flashStore, "error", "New passwords do not match")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Validate password length
	if len(newPassword) < 12 {
		h.sessionManager.AddFlash(r, h.flashStore, "error", "Password must be at least 12 characters")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Get user to verify current password
	user, err := h.userRepo.GetByID(session.UserID)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": session.UserID}).Error("Failed to get user")
		http.Error(w, "Failed to load user", http.StatusInternalServerError)
		return
	}

	// Verify current password by trying to authenticate
	if _, err := h.userRepo.Authenticate(user.Email, currentPassword); err != nil {
		h.sessionManager.AddFlash(r, h.flashStore, "error", "Current password is incorrect")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Change password
	if err := h.userRepo.ChangePassword(session.UserID, newPassword, 12); err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": session.UserID}).Error("Failed to change password")
		h.sessionManager.AddFlash(r, h.flashStore, "error", "Failed to change password")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	log.WithFields(log.Fields{"user_id": session.UserID}).Info("User changed password")
	h.sessionManager.AddFlash(r, h.flashStore, "success", "Password changed successfully")
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

// RevokeSession revokes a specific session
func (h *Handlers) RevokeSession(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Unauthorized"})
		return
	}

	sessionID := ps.ByName("id")

	// Verify the session belongs to the current user
	targetSession, err := h.sessionRepo.Get(sessionID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Session not found"})
		return
	}

	if targetSession.UserID != session.UserID {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Forbidden"})
		return
	}

	// Prevent revoking current session
	if sessionID == session.ID {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Cannot revoke current session"})
		return
	}

	// Delete the session
	if err := h.sessionRepo.Delete(sessionID); err != nil {
		log.WithFields(log.Fields{"error": err, "session_id": sessionID}).Error("Failed to delete session")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to revoke session"})
		return
	}

	log.WithFields(log.Fields{"user_id": session.UserID, "revoked_session": sessionID}).Info("User revoked session")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// PasswordResetRequestPage shows the password reset request form
// PasswordResetRequestPage shows the password reset request form
func (h *Handlers) PasswordResetRequestPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	data := h.sessionManager.NewTemplateData(r, h.flashStore, "Reset Password")
	if err := h.render(w, "password_reset_request.html", data); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to render password reset request page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// PasswordResetRequestPost handles password reset request submission
func (h *Handlers) PasswordResetRequestPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/password-reset", http.StatusSeeOther)
		return
	}

	emailAddr := r.FormValue("email")

	// Look up user by email
	user, err := h.userRepo.GetByEmail(emailAddr)
	if err != nil {
		// Don't reveal if email exists or not (timing attack prevention)
		log.WithFields(log.Fields{"email": emailAddr}).Debug("Password reset requested for non-existent email")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Delete any existing password reset tokens for this user
	if err := h.passwordResetRepo.DeleteByUserID(user.ID); err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": user.ID}).Warn("Failed to delete old password reset tokens")
	}

	// Create password reset token (valid for 1 hour)
	resetToken, err := h.passwordResetRepo.Create(user.ID, emailAddr, 1)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "email": emailAddr}).Error("Failed to create password reset token")
		http.Redirect(w, r, "/password-reset", http.StatusSeeOther)
		return
	}

	// Send password reset email
	resetURL := fmt.Sprintf("%s/password-reset/%s", h.baseURL, resetToken.Token)
	subject, body := email.PasswordResetEmail(emailAddr, resetToken.Token, resetURL)

	if err := h.mailer.SendEmail(emailAddr, subject, body); err != nil {
		log.WithFields(log.Fields{"error": err, "email": emailAddr}).Error("Failed to send password reset email")
	} else {
		log.WithFields(log.Fields{"email": emailAddr, "user_id": user.ID}).Info("Password reset email sent")
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// PasswordResetPage shows the password reset form
func (h *Handlers) PasswordResetPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	token := ps.ByName("token")

	data := h.sessionManager.NewTemplateData(r, h.flashStore, "Set New Password")
	data.Data = map[string]interface{}{
		"Token":             token,
		"MinPasswordLength": h.config.MinPasswordLength,
	}

	// Validate token
	_, err := h.passwordResetRepo.GetValid(token)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "token": token}).Warn("Invalid password reset token")
		data.Data["Error"] = "This password reset link is invalid or has expired."
	}

	if err := h.render(w, "password_reset.html", data); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to render password reset page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// PasswordResetPost handles password reset form submission
func (h *Handlers) PasswordResetPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	token := ps.ByName("token")

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/password-reset/"+token, http.StatusSeeOther)
		return
	}

	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	// Validate passwords match
	if password != passwordConfirm {
		http.Redirect(w, r, "/password-reset/"+token, http.StatusSeeOther)
		return
	}

	// Validate password length
	if len(password) < h.config.MinPasswordLength {
		http.Redirect(w, r, "/password-reset/"+token, http.StatusSeeOther)
		return
	}

	// Validate and get token
	resetToken, err := h.passwordResetRepo.GetValid(token)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "token": token}).Warn("Invalid password reset token on submission")
		http.Redirect(w, r, "/password-reset", http.StatusSeeOther)
		return
	}

	// Change password
	if err := h.userRepo.ChangePassword(resetToken.UserID, password, 12); err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": resetToken.UserID}).Error("Failed to change password")
		http.Redirect(w, r, "/password-reset/"+token, http.StatusSeeOther)
		return
	}

	// Mark token as used
	if err := h.passwordResetRepo.MarkUsed(token); err != nil {
		log.WithFields(log.Fields{"error": err, "token": token}).Warn("Failed to mark reset token as used")
	}

	log.WithFields(log.Fields{"user_id": resetToken.UserID, "email": resetToken.Email}).Info("Password reset successfully")

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
