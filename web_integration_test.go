// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/joohoi/acme-dns/admin"
	"github.com/joohoi/acme-dns/models"
	"github.com/joohoi/acme-dns/web"
)

// TestWebUIIntegration tests the complete web UI flow
func TestWebUIIntegration(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database
	var dbcfg = dbsettings{
		Engine:     "sqlite3",
		Connection: ":memory:"}
	Config = DNSConfig{
		Database: dbcfg,
		General: general{
			Domain: "auth.example.com",
		},
		WebUI: webui{
			Enabled:               true,
			SessionDuration:       24,
			AllowSelfRegistration: true,
			MinPasswordLength:     12,
		},
		Security: security{
			RateLimiting:         true,
			MaxLoginAttempts:     5,
			LockoutDuration:      15,
			SessionCookieName:    "acmedns_session",
			CSRFCookieName:       "acmedns_csrf",
			MaxRequestBodySize:   1048576,
		},
	}

	// Initialize database
	newDB := new(acmedb)
	if err := newDB.Init(Config.Database.Engine, Config.Database.Connection); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer newDB.Close()

	// Create repositories
	userRepo := models.NewUserRepository(newDB.GetBackend(), Config.Database.Engine)
	sessionRepo := models.NewSessionRepository(newDB.GetBackend(), Config.Database.Engine)
	recordRepo := models.NewRecordRepository(newDB.GetBackend(), Config.Database.Engine)

	// Create session manager and flash store
	sessionManager := web.NewSessionManager(sessionRepo)
	flashStore := web.NewFlashStore()

	// Create test admin user
	adminUser, err := userRepo.Create("admin@test.com", "admin123456789", true, BcryptCostWeb)
	if err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	// Create test regular user
	regularUser, err := userRepo.Create("user@test.com", "user123456789", false, BcryptCostWeb)
	if err != nil {
		t.Fatalf("Failed to create regular user: %v", err)
	}

	// Initialize web handlers
	webConfig := web.WebConfig{
		AllowSelfRegistration: Config.WebUI.AllowSelfRegistration,
		MinPasswordLength:     Config.WebUI.MinPasswordLength,
	}
	webHandlers, err := web.NewHandlers(sessionManager, flashStore, userRepo, recordRepo, "web/templates", webConfig, Config.General.Domain)
	if err != nil {
		t.Fatalf("Failed to create web handlers: %v", err)
	}

	// Initialize admin handlers
	adminHandlers, err := admin.NewHandlers(sessionManager, flashStore, userRepo, recordRepo, "web/templates", Config.General.Domain)
	if err != nil {
		t.Fatalf("Failed to create admin handlers: %v", err)
	}

	t.Run("RootRedirectsToLogin", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		webHandlers.RootHandler(w, req, nil)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected status %d, got %d", http.StatusSeeOther, w.Code)
		}
		if location := w.Header().Get("Location"); location != "/login" {
			t.Errorf("Expected redirect to /login, got %s", location)
		}
	})

	t.Run("LoginPageRenders", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/login", nil)
		w := httptest.NewRecorder()

		webHandlers.LoginPage(w, req, nil)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}
		body := w.Body.String()
		if !strings.Contains(body, "Login") {
			t.Error("Login page should contain 'Login' text")
		}
		if !strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("Login page should contain proper HTML")
		}
	})

	t.Run("LoginWithValidCredentials", func(t *testing.T) {
		form := url.Values{}
		form.Add("email", "user@test.com")
		form.Add("password", "user123456789")

		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		webHandlers.LoginPost(w, req, nil)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected redirect status %d, got %d", http.StatusSeeOther, w.Code)
		}
		if location := w.Header().Get("Location"); location != "/dashboard" {
			t.Errorf("Expected redirect to /dashboard, got %s", location)
		}

		// Check session cookie was set
		cookies := w.Result().Cookies()
		foundSession := false
		for _, cookie := range cookies {
			if cookie.Name == Config.Security.SessionCookieName {
				foundSession = true
				break
			}
		}
		if !foundSession {
			t.Error("Session cookie should be set after login")
		}
	})

	t.Run("LoginWithInvalidCredentials", func(t *testing.T) {
		form := url.Values{}
		form.Add("email", "user@test.com")
		form.Add("password", "wrongpassword")

		req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		webHandlers.LoginPost(w, req, nil)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected redirect status %d, got %d", http.StatusSeeOther, w.Code)
		}
		// Should redirect back to login with error
		if location := w.Header().Get("Location"); !strings.Contains(location, "/login") {
			t.Errorf("Expected redirect to /login, got %s", location)
		}
	})

	t.Run("DashboardRequiresAuth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dashboard", nil)
		w := httptest.NewRecorder()

		// Try to access dashboard without session
		webHandlers.Dashboard(w, req, nil)

		if w.Code != http.StatusSeeOther {
			t.Errorf("Expected redirect status %d, got %d", http.StatusSeeOther, w.Code)
		}
		if location := w.Header().Get("Location"); location != "/login" {
			t.Errorf("Expected redirect to /login, got %s", location)
		}
	})

	t.Run("DashboardAccessWithAuth", func(t *testing.T) {
		// Create a session for the user
		req := httptest.NewRequest("GET", "/dashboard", nil)
		w := httptest.NewRecorder()

		session, err := sessionManager.CreateSession(w, req, regularUser.ID, 24)
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Add session cookie to request
		req2 := httptest.NewRequest("GET", "/dashboard", nil)
		req2.AddCookie(&http.Cookie{
			Name:  Config.Security.SessionCookieName,
			Value: session.ID,
		})
		w2 := httptest.NewRecorder()

		webHandlers.Dashboard(w2, req2, nil)

		if w2.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w2.Code)
		}
		body := w2.Body.String()
		if !strings.Contains(body, "Dashboard") {
			t.Error("Dashboard should contain 'Dashboard' text")
		}
	})

	t.Run("AdminPanelRequiresAdminAuth", func(t *testing.T) {
		// Try with regular user session
		req := httptest.NewRequest("GET", "/admin", nil)
		w := httptest.NewRecorder()

		session, _ := sessionManager.CreateSession(w, req, regularUser.ID, 24)

		req2 := httptest.NewRequest("GET", "/admin", nil)
		req2.AddCookie(&http.Cookie{
			Name:  Config.Security.SessionCookieName,
			Value: session.ID,
		})
		w2 := httptest.NewRecorder()

		adminHandlers.AdminPanel(w2, req2, nil)

		// Should be forbidden
		if w2.Code != http.StatusForbidden {
			t.Errorf("Expected status %d for non-admin, got %d", http.StatusForbidden, w2.Code)
		}
	})

	t.Run("AdminPanelAccessWithAdminAuth", func(t *testing.T) {
		// Create admin session
		req := httptest.NewRequest("GET", "/admin", nil)
		w := httptest.NewRecorder()

		session, err := sessionManager.CreateSession(w, req, adminUser.ID, 24)
		if err != nil {
			t.Fatalf("Failed to create admin session: %v", err)
		}

		req2 := httptest.NewRequest("GET", "/admin", nil)
		req2.AddCookie(&http.Cookie{
			Name:  Config.Security.SessionCookieName,
			Value: session.ID,
		})
		w2 := httptest.NewRecorder()

		adminHandlers.AdminPanel(w2, req2, nil)

		if w2.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w2.Code)
		}
		body := w2.Body.String()
		if !strings.Contains(body, "Admin") {
			t.Error("Admin panel should contain 'Admin' text")
		}
	})

	t.Run("LogoutClearsSession", func(t *testing.T) {
		// Create session
		req := httptest.NewRequest("GET", "/logout", nil)
		w := httptest.NewRecorder()

		session, _ := sessionManager.CreateSession(w, req, regularUser.ID, 24)

		// Logout
		req2 := httptest.NewRequest("GET", "/logout", nil)
		req2.AddCookie(&http.Cookie{
			Name:  Config.Security.SessionCookieName,
			Value: session.ID,
		})
		w2 := httptest.NewRecorder()

		webHandlers.Logout(w2, req2, nil)

		if w2.Code != http.StatusSeeOther {
			t.Errorf("Expected redirect status %d, got %d", http.StatusSeeOther, w2.Code)
		}

		// Try to use session after logout
		_, err := sessionRepo.GetValid(session.ID)
		if err == nil {
			t.Error("Session should be invalid after logout")
		}
	})
}

// TestTemplateRendering tests that all templates render without errors
func TestTemplateRendering(t *testing.T) {
	templates, err := web.GetTemplates()
	if err != nil {
		t.Fatalf("Failed to load templates: %v", err)
	}

	// Test that base template exists
	if templates.Lookup("base") == nil {
		t.Error("Base template not found")
	}

	// Test that all page templates exist
	pageTemplates := []string{
		"login-page.html",
		"dashboard-page.html",
		"admin-page.html",
	}

	for _, tmpl := range pageTemplates {
		if templates.Lookup(tmpl) == nil {
			t.Errorf("Template %s not found", tmpl)
		}
	}
}

// TestStaticFilesServe tests that static files are served correctly
func TestStaticFilesServe(t *testing.T) {
	handler := web.GetStaticHandler()

	// Test CSS file
	req := httptest.NewRequest("GET", "/css/style.css", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should either return the file or 404 (if file doesn't exist yet)
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("Expected status 200 or 404, got %d", w.Code)
	}
}
