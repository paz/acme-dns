package admin

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"

	"github.com/joohoi/acme-dns/email"
	"github.com/joohoi/acme-dns/models"
	"github.com/joohoi/acme-dns/web"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// Handlers holds dependencies for admin handlers
type Handlers struct {
	sessionManager    *web.SessionManager
	flashStore        *web.FlashStore
	userRepo          UserRepository
	recordRepo        RecordRepository
	passwordResetRepo *models.PasswordResetRepository
	mailer            *email.Mailer
	templates         *template.Template
	domain            string
	baseURL           string
}

// UserRepository interface for user operations
type UserRepository interface {
	GetByID(id int64) (*models.User, error)
	ListAll(activeOnly bool) ([]*models.User, error)
	Create(email, password string, isAdmin bool, bcryptCost int) (*models.User, error)
	Delete(userID int64) error
	SetActive(userID int64, active bool) error
}

// RecordRepository interface for record operations
type RecordRepository interface {
	ListAll() ([]*models.Record, error)
	ListUnmanaged() ([]*models.Record, error)
	ClaimRecord(username string, userID int64, description string) error
	DeleteByAdmin(username string) error
}

// NewHandlers creates new admin handlers
func NewHandlers(
	sm *web.SessionManager,
	fs *web.FlashStore,
	userRepo UserRepository,
	recordRepo RecordRepository,
	passwordResetRepo *models.PasswordResetRepository,
	mailer *email.Mailer,
	templatesDir string, // Kept for backward compatibility but not used
	domain string,
	baseURL string,
) (*Handlers, error) {
	// Load templates from embedded filesystem
	templates, err := web.GetTemplates()
	if err != nil {
		return nil, err
	}

	return &Handlers{
		sessionManager:    sm,
		flashStore:        fs,
		userRepo:          userRepo,
		recordRepo:        recordRepo,
		passwordResetRepo: passwordResetRepo,
		mailer:            mailer,
		templates:         templates,
		domain:            domain,
		baseURL:           baseURL,
	}, nil
}

// render executes a template by name
func (h *Handlers) render(w http.ResponseWriter, templateName string, data *web.TemplateData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Map template names to their content block names
	contentBlockMap := map[string]string{
		"login.html":     "login-content",
		"dashboard.html": "dashboard-content",
		"profile.html":   "profile-content",
		"register.html":  "register-content",
		"admin.html":     "admin-content",
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

// Dashboard displays the admin dashboard
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get admin user info
	user, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !user.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get statistics
	users, err := h.userRepo.ListAll(false)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to list users")
		http.Error(w, "Failed to load users", http.StatusInternalServerError)
		return
	}

	records, err := h.recordRepo.ListAll()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to list records")
		http.Error(w, "Failed to load records", http.StatusInternalServerError)
		return
	}

	unmanagedRecords, err := h.recordRepo.ListUnmanaged()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to list unmanaged records")
		unmanagedRecords = []*models.Record{}
	}

	// Prepare template data
	data := h.sessionManager.NewTemplateData(r, h.flashStore, "Admin Dashboard")
	data.User = user
	data.IsAdmin = true
	data.Data["Users"] = users
	data.Data["Records"] = records
	data.Data["UnmanagedRecords"] = unmanagedRecords
	data.Data["Domain"] = h.domain
	data.Data["Stats"] = map[string]interface{}{
		"TotalUsers":      len(users),
		"TotalRecords":    len(records),
		"UnmanagedCount":  len(unmanagedRecords),
		"ManagedCount":    len(records) - len(unmanagedRecords),
	}

	if err := h.render(w, "admin.html", data); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to render admin template")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// ListUsers returns a JSON list of all users
func (h *Handlers) ListUsers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !user.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	users, err := h.userRepo.ListAll(false)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to list users")
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(users); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// CreateUser creates a new user
func (h *Handlers) CreateUser(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Unauthorized"})
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Forbidden"})
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid form data"})
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	passwordMethod := r.FormValue("password_method")
	isAdminStr := r.FormValue("is_admin")
	isAdmin := isAdminStr == "true" || isAdminStr == "1"

	var newUser *models.User

	if passwordMethod == "email" {
		// Generate temporary password and send email
		tempPassword := generateSecurePassword(16)

		// Create user with temporary password
		newUser, err = h.userRepo.Create(email, tempPassword, isAdmin, 12)
		if err != nil {
			log.WithFields(log.Fields{"error": err, "email": email}).Error("Failed to create user")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to create user: " + err.Error()})
			return
		}

		// Create password reset token
		resetObj, err := h.passwordResetRepo.Create(newUser.ID, email, 24)
		if err != nil {
			log.WithFields(log.Fields{"error": err, "user_id": newUser.ID}).Error("Failed to create password reset token")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to create password reset token"})
			return
		}

		// Send password reset email
		resetURL := fmt.Sprintf("%s/reset-password?token=%s", h.baseURL, resetObj.Token)
		subject := "Set Your Password - acme-dns"
		// HTML-escape the URL to prevent any potential injection
		safeResetURL := html.EscapeString(resetURL)
		body := fmt.Sprintf(`
			<html>
			<body>
				<h2>Welcome to acme-dns!</h2>
				<p>An administrator has created an account for you. Please set your password by clicking the link below:</p>
				<p><a href="%s">Set Password</a></p>
				<p>This link will expire in 24 hours.</p>
				<p>If you did not request this account, please ignore this email.</p>
			</body>
			</html>
		`, safeResetURL)

		if err := h.mailer.SendEmail(email, subject, body); err != nil {
			log.WithFields(log.Fields{"error": err, "email": email}).Error("Failed to send password reset email")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "User created but failed to send email"})
			return
		}

		log.WithFields(log.Fields{
			"admin_id":    session.UserID,
			"new_user_id": newUser.ID,
			"email":       email,
			"is_admin":    isAdmin,
			"method":      "email",
		}).Info("Admin created new user with email password reset")
	} else {
		// Manual password set
		if password == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Password is required for manual setup"})
			return
		}

		newUser, err = h.userRepo.Create(email, password, isAdmin, 12)
		if err != nil {
			log.WithFields(log.Fields{"error": err, "email": email}).Error("Failed to create user")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to create user: " + err.Error()})
			return
		}

		log.WithFields(log.Fields{
			"admin_id":    session.UserID,
			"new_user_id": newUser.ID,
			"email":       email,
			"is_admin":    isAdmin,
			"method":      "manual",
		}).Info("Admin created new user with manual password")
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"user":   newUser,
	}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// DeleteUser deletes a user
func (h *Handlers) DeleteUser(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	userIDStr := ps.ByName("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Prevent admin from deleting themselves
	if userID == session.UserID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	err = h.userRepo.Delete(userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": userID}).Error("Failed to delete user")
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{
		"admin_id":      session.UserID,
		"deleted_user_id": userID,
	}).Info("Admin deleted user")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// ResetUserPassword sends a password reset email to a user
func (h *Handlers) ResetUserPassword(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Unauthorized"})
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Forbidden"})
		return
	}

	userIDStr := ps.ByName("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid user ID"})
		return
	}

	// Get target user
	targetUser, err := h.userRepo.GetByID(userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": userID}).Error("Failed to get user for password reset")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to get user"})
		return
	}

	// Create password reset token
	resetObj, err := h.passwordResetRepo.Create(targetUser.ID, targetUser.Email, 24)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": targetUser.ID}).Error("Failed to create password reset token")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to create password reset token"})
		return
	}

	// Send password reset email
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", h.baseURL, resetObj.Token)
	subject := "Password Reset - acme-dns"
	// HTML-escape the URL to prevent any potential injection
	safeResetURL := html.EscapeString(resetURL)
	body := fmt.Sprintf(`
		<html>
		<body>
			<h2>Password Reset Request</h2>
			<p>An administrator has initiated a password reset for your account. Click the link below to reset your password:</p>
			<p><a href="%s">Reset Password</a></p>
			<p>This link will expire in 24 hours.</p>
			<p>If you did not request this password reset, please contact your administrator.</p>
		</body>
		</html>
	`, safeResetURL)

	if err := h.mailer.SendEmail(targetUser.Email, subject, body); err != nil {
		log.WithFields(log.Fields{"error": err, "email": targetUser.Email}).Error("Failed to send password reset email")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to send email"})
		return
	}

	log.WithFields(log.Fields{
		"admin_id":      session.UserID,
		"target_user_id": targetUser.ID,
		"email":         targetUser.Email,
	}).Info("Admin sent password reset email to user")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Password reset email sent"})
}

// ToggleUserActive toggles a user's active status
func (h *Handlers) ToggleUserActive(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	userIDStr := ps.ByName("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	activeStr := r.FormValue("active")
	active := activeStr == "true" || activeStr == "1"

	err = h.userRepo.SetActive(userID, active)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "user_id": userID}).Error("Failed to toggle user active status")
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{
		"admin_id": session.UserID,
		"user_id":  userID,
		"active":   active,
	}).Info("Admin toggled user active status")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// ListDomains returns a JSON list of all domains
func (h *Handlers) ListDomains(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	records, err := h.recordRepo.ListAll()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to list records")
		http.Error(w, "Failed to list records", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(records); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// ListUnmanagedDomains returns a JSON list of unmanaged domains
func (h *Handlers) ListUnmanagedDomains(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	records, err := h.recordRepo.ListUnmanaged()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to list unmanaged records")
		http.Error(w, "Failed to list unmanaged records", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(records); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// ClaimDomain assigns an unmanaged domain to a user
func (h *Handlers) ClaimDomain(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Unauthorized"})
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Forbidden"})
		return
	}

	username := ps.ByName("username")

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid form data"})
		return
	}

	userIDStr := r.FormValue("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid user ID"})
		return
	}

	description := r.FormValue("description")

	err = h.recordRepo.ClaimRecord(username, userID, description)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "username": username, "user_id": userID}).Error("Failed to claim record")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Failed to claim domain: " + err.Error()})
		return
	}

	log.WithFields(log.Fields{
		"admin_id":  session.UserID,
		"username":  username,
		"user_id":   userID,
	}).Info("Admin claimed domain for user")

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// DeleteDomain deletes any domain (admin override)
func (h *Handlers) DeleteDomain(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	username := ps.ByName("username")

	err = h.recordRepo.DeleteByAdmin(username)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "username": username}).Error("Failed to delete domain")
		http.Error(w, "Failed to delete domain", http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{
		"admin_id": session.UserID,
		"username": username,
	}).Info("Admin deleted domain")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// BulkClaimDomains claims multiple domains for a user
func (h *Handlers) BulkClaimDomains(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Unauthorized"})
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Forbidden"})
		return
	}

	// Parse JSON request body
	var req struct {
		Usernames   []string `json:"usernames"`
		UserID      int64    `json:"user_id"`
		Description string   `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid request body"})
		return
	}

	if len(req.Usernames) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "No usernames provided"})
		return
	}

	if req.UserID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "User ID is required"})
		return
	}

	// Process each domain
	var successCount, failCount int
	var errors []string

	for _, username := range req.Usernames {
		err = h.recordRepo.ClaimRecord(username, req.UserID, req.Description)
		if err != nil {
			failCount++
			errors = append(errors, username+": "+err.Error())
			log.WithFields(log.Fields{"error": err, "username": username, "user_id": req.UserID}).Error("Failed to claim record in bulk operation")
		} else {
			successCount++
		}
	}

	log.WithFields(log.Fields{
		"admin_id":      session.UserID,
		"user_id":       req.UserID,
		"success_count": successCount,
		"fail_count":    failCount,
		"total":         len(req.Usernames),
	}).Info("Admin bulk claimed domains")

	response := map[string]interface{}{
		"status":        "success",
		"success_count": successCount,
		"fail_count":    failCount,
		"total":         len(req.Usernames),
	}

	if failCount > 0 {
		response["errors"] = errors
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// BulkDeleteDomains deletes multiple domains
func (h *Handlers) BulkDeleteDomains(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")

	session, err := h.sessionManager.GetSession(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Unauthorized"})
		return
	}

	adminUser, err := h.userRepo.GetByID(session.UserID)
	if err != nil || !adminUser.IsAdmin {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Forbidden"})
		return
	}

	// Parse JSON request body
	var req struct {
		Usernames []string `json:"usernames"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Invalid request body"})
		return
	}

	if len(req.Usernames) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "No usernames provided"})
		return
	}

	// Process each domain
	var successCount, failCount int
	var errors []string

	for _, username := range req.Usernames {
		err = h.recordRepo.DeleteByAdmin(username)
		if err != nil {
			failCount++
			errors = append(errors, username+": "+err.Error())
			log.WithFields(log.Fields{"error": err, "username": username}).Error("Failed to delete domain in bulk operation")
		} else {
			successCount++
		}
	}

	log.WithFields(log.Fields{
		"admin_id":      session.UserID,
		"success_count": successCount,
		"fail_count":    failCount,
		"total":         len(req.Usernames),
	}).Info("Admin bulk deleted domains")

	response := map[string]interface{}{
		"status":        "success",
		"success_count": successCount,
		"fail_count":    failCount,
		"total":         len(req.Usernames),
	}

	if failCount > 0 {
		response["errors"] = errors
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to encode JSON response")
	}
}

// generateSecurePassword generates a cryptographically secure random password
func generateSecurePassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Failed to generate secure password")
		// Fallback to a default secure password (this should never happen)
		return "ChangeMe123456!!"
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}
