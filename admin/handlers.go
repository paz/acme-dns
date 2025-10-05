package admin

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"

	"github.com/joohoi/acme-dns/models"
	"github.com/joohoi/acme-dns/web"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// Handlers holds dependencies for admin handlers
type Handlers struct {
	sessionManager *web.SessionManager
	flashStore     *web.FlashStore
	userRepo       UserRepository
	recordRepo     RecordRepository
	templates      *template.Template
	domain         string
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
	templatesDir string, // Kept for backward compatibility but not used
	domain string,
) (*Handlers, error) {
	// Load templates from embedded filesystem
	templates, err := web.GetTemplates()
	if err != nil {
		return nil, err
	}

	return &Handlers{
		sessionManager: sm,
		flashStore:     fs,
		userRepo:       userRepo,
		recordRepo:     recordRepo,
		templates:      templates,
		domain:         domain,
	}, nil
}

// render executes a template by name
func (h *Handlers) render(w http.ResponseWriter, templateName string, data *web.TemplateData) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return h.templates.ExecuteTemplate(w, templateName, data)
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

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")
	isAdminStr := r.FormValue("is_admin")
	isAdmin := isAdminStr == "true" || isAdminStr == "1"

	newUser, err := h.userRepo.Create(email, password, isAdmin, 12)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "email": email}).Error("Failed to create user")
		http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{
		"admin_id":     session.UserID,
		"new_user_id":  newUser.ID,
		"email":        email,
		"is_admin":     isAdmin,
	}).Info("Admin created new user")

	w.Header().Set("Content-Type", "application/json")
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

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	userIDStr := r.FormValue("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	description := r.FormValue("description")

	err = h.recordRepo.ClaimRecord(username, userID, description)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "username": username, "user_id": userID}).Error("Failed to claim record")
		http.Error(w, "Failed to claim domain: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.WithFields(log.Fields{
		"admin_id":  session.UserID,
		"username":  username,
		"user_id":   userID,
	}).Info("Admin claimed domain for user")

	w.Header().Set("Content-Type", "application/json")
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
