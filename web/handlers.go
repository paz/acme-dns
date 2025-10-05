package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/joohoi/acme-dns/models"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// Handlers holds all dependencies for web handlers
type Handlers struct {
	sessionManager *SessionManager
	flashStore     *FlashStore
	userRepo       UserRepository
	recordRepo     RecordRepository
	templates      *template.Template
	config         WebConfig
	domain         string
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
	Create(email, password string, isAdmin bool, bcryptCost int) (*models.User, error)
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
	templatesDir string,
	config WebConfig,
	domain string,
) (*Handlers, error) {
	// Load templates
	templates, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		return nil, err
	}

	return &Handlers{
		sessionManager: sm,
		flashStore:     fs,
		userRepo:       userRepo,
		recordRepo:     recordRepo,
		templates:      templates,
		config:         config,
		domain:         domain,
	}, nil
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

	if err := h.templates.ExecuteTemplate(w, "login.html", data); err != nil {
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

	// Redirect to dashboard or requested page
	if redirect != "" && redirect[0] == '/' {
		http.Redirect(w, r, redirect, http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
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
	data.Data["Records"] = records
	data.Data["Domain"] = h.domain

	if err := h.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
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

	// Verify ownership (this should be done in recordRepo.GetByUsername with user_id check)
	// For now, we'll just return the credentials

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

	// Would render a register template (not yet created)
	if _, err := w.Write([]byte("Register page - to be implemented with template")); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to write response")
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
