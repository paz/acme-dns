//go:build !test
// +build !test

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/caddyserver/certmagic"
	legolog "github.com/go-acme/lego/v4/log"
	"github.com/joohoi/acme-dns/admin"
	"github.com/joohoi/acme-dns/email"
	"github.com/joohoi/acme-dns/models"
	"github.com/joohoi/acme-dns/web"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Created files are not world writable (Unix-only)
	// Note: syscall.Umask is not available on Windows
	// This is handled by file permissions in Windows differently

	// CLI flags
	configPtr := flag.String("c", "/etc/acme-dns/config.cfg", "config file location")
	createAdminPtr := flag.String("create-admin", "", "create admin user with specified email")
	versionPtr := flag.Bool("version", false, "show version information")
	dbInfoPtr := flag.Bool("db-info", false, "show database migration status")

	flag.Parse()

	// Handle version flag
	if *versionPtr {
		ShowVersion()
		os.Exit(0)
	}
	// Read global config
	var err error
	if fileIsAccessible(*configPtr) {
		log.WithFields(log.Fields{"file": *configPtr}).Info("Using config file")
		Config, err = readConfig(*configPtr)
	} else if fileIsAccessible("./config.cfg") {
		log.WithFields(log.Fields{"file": "./config.cfg"}).Info("Using config file")
		Config, err = readConfig("./config.cfg")
	} else {
		log.Errorf("Configuration file not found.")
		os.Exit(1)
	}
	if err != nil {
		log.Errorf("Encountered an error while trying to read configuration file:  %s", err)
		os.Exit(1)
	}

	setupLogging(Config.Logconfig.Format, Config.Logconfig.Level)

	// Handle database info flag
	if *dbInfoPtr {
		if err := ShowDatabaseInfo(); err != nil {
			log.Errorf("Error getting database info: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle create admin flag
	if *createAdminPtr != "" {
		if err := CreateAdminUser(*createAdminPtr); err != nil {
			log.Error("Error creating admin user")
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Open database
	newDB := new(acmedb)
	err = newDB.Init(Config.Database.Engine, Config.Database.Connection)
	if err != nil {
		log.Errorf("Could not open database [%v]", err)
		os.Exit(1)
	} else {
		log.Info("Connected to database")
	}
	DB = newDB
	defer DB.Close()

	// Error channel for servers
	errChan := make(chan error, 1)

	// DNS server
	dnsservers := make([]*DNSServer, 0)
	if strings.HasPrefix(Config.General.Proto, "both") {
		// Handle the case where DNS server should be started for both udp and tcp
		udpProto := "udp"
		tcpProto := "tcp"
		if strings.HasSuffix(Config.General.Proto, "4") {
			udpProto += "4"
			tcpProto += "4"
		} else if strings.HasSuffix(Config.General.Proto, "6") {
			udpProto += "6"
			tcpProto += "6"
		}
		dnsServerUDP := NewDNSServer(DB, Config.General.Listen, udpProto, Config.General.Domain)
		dnsservers = append(dnsservers, dnsServerUDP)
		dnsServerUDP.ParseRecords(Config)
		dnsServerTCP := NewDNSServer(DB, Config.General.Listen, tcpProto, Config.General.Domain)
		dnsservers = append(dnsservers, dnsServerTCP)
		// No need to parse records from config again
		dnsServerTCP.Domains = dnsServerUDP.Domains
		dnsServerTCP.SOA = dnsServerUDP.SOA
		go dnsServerUDP.Start(errChan)
		go dnsServerTCP.Start(errChan)
	} else {
		dnsServer := NewDNSServer(DB, Config.General.Listen, Config.General.Proto, Config.General.Domain)
		dnsservers = append(dnsservers, dnsServer)
		dnsServer.ParseRecords(Config)
		go dnsServer.Start(errChan)
	}

	// HTTP API
	go startHTTPAPI(errChan, Config, dnsservers)

	// block waiting for error
	for {
		err = <-errChan
		if err != nil {
			log.Fatal(err)
		}
	}
}

func startHTTPAPI(errChan chan error, config DNSConfig, dnsservers []*DNSServer) {
	// Setup http logger
	logger := log.New()
	logwriter := logger.Writer()
	defer logwriter.Close()
	// Setup logging for different dependencies to log with logrus
	// Certmagic
	stdlog.SetOutput(logwriter)
	// Lego
	legolog.Logger = logger

	api := httprouter.New()
	c := cors.New(cors.Options{
		AllowedOrigins:     Config.API.CorsOrigins,
		AllowedMethods:     []string{"GET", "POST"},
		OptionsPassthrough: false,
		Debug:              Config.General.Debug,
	})
	if Config.General.Debug {
		// Logwriter for saner log output
		c.Log = stdlog.New(logwriter, "", 0)
	}
	// API endpoints (existing, backward compatible)
	if !Config.API.DisableRegistration {
		api.POST("/register", webRegisterPost)
	}
	api.POST("/update", Auth(webUpdatePost))
	api.GET("/health", healthCheck)

	// Web UI endpoints (only if enabled)
	if Config.WebUI.Enabled {
		log.Info("Web UI enabled - initializing web components")

		// Initialize repositories
		userRepo := models.NewUserRepository(DB.GetBackend(), Config.Database.Engine)
		sessionRepo := models.NewSessionRepository(DB.GetBackend(), Config.Database.Engine)
		recordRepo := models.NewRecordRepository(DB.GetBackend(), Config.Database.Engine)
		passwordResetRepo := models.NewPasswordResetRepository(DB.GetBackend())

		// Initialize email mailer
		emailConfig := email.Config{
			Enabled:     Config.Email.Enabled,
			SMTPHost:    Config.Email.SMTPHost,
			SMTPPort:    Config.Email.SMTPPort,
			SMTPUser:    Config.Email.SMTPUser,
			SMTPPass:    Config.Email.SMTPPass,
			FromEmail:   Config.Email.FromEmail,
			FromName:    Config.Email.FromName,
			UseTLS:      Config.Email.UseTLS,
			UseStartTLS: Config.Email.UseStartTLS,
		}
		mailer := email.NewMailer(emailConfig)

		// Create session manager
		sessionManager := web.NewSessionManager(
			sessionRepo,
			Config.Security.SessionCookieName,
			Config.API.TLS != "", // Secure cookies if TLS is enabled
		)

		// Create flash message store
		flashStore := web.NewFlashStore()

		// Initialize rate limiter for web UI
		webRateLimiter := web.NewRateLimiter(60, 10) // 60 requests/min, burst 10
		webRateLimiter.Cleanup()

		// Start session cleanup goroutine
		go func() {
			for {
				if err := sessionRepo.DeleteExpired(); err != nil {
					log.WithFields(log.Fields{"error": err}).Warn("Session cleanup failed")
				}
				log.Debug("Cleaned up expired sessions")
				// Run every hour
				<-time.After(1 * time.Hour)
			}
		}()

		// Initialize web handlers
		webConfig := web.WebConfig{
			AllowSelfRegistration: Config.WebUI.AllowSelfRegistration,
			MinPasswordLength:     Config.WebUI.MinPasswordLength,
		}
		// Build base URL for password reset emails
		protocol := "https"
		if Config.API.TLS == "none" || Config.API.TLS == "" {
			protocol = "http"
		}
		baseURL := fmt.Sprintf("%s://%s", protocol, Config.General.Domain)

		webHandlers, err := web.NewHandlers(
			sessionManager,
			flashStore,
			userRepo,
			recordRepo,
			sessionRepo,
			passwordResetRepo,
			mailer,
			"web/templates",
			webConfig,
			Config.General.Domain,
			baseURL,
		)
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("Failed to initialize web handlers")
		} else {
			// Initialize admin handlers
			adminHandlers, err := admin.NewHandlers(
				sessionManager,
				flashStore,
				userRepo,
				recordRepo,
				"web/templates",
				Config.General.Domain,
			)
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Error("Failed to initialize admin handlers")
			} else {
				// Serve static files from embedded filesystem
				api.Handler("GET", "/static/*filepath", http.StripPrefix("/static", web.GetStaticHandler()))

				// Root route
				api.GET("/", web.ChainMiddleware(
					webHandlers.RootHandler,
					web.LoggingMiddleware,
				))

				// Public routes
				api.GET("/login", web.ChainMiddleware(
					webHandlers.LoginPage,
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.POST("/login", web.ChainMiddleware(
					webHandlers.LoginPost,
					web.SecurityHeadersMiddleware,
					web.RequestSizeLimitMiddleware(int64(Config.Security.MaxRequestBodySize)),
					web.RateLimitMiddleware(webRateLimiter, Config.Security.RateLimiting),
					web.LoggingMiddleware,
				))
				api.GET("/logout", web.ChainMiddleware(
					webHandlers.Logout,
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))

				// User routes (authentication required)
				api.GET("/dashboard", web.ChainMiddleware(
					webHandlers.Dashboard,
					web.RequireAuth(sessionManager),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.POST("/dashboard/register", web.ChainMiddleware(
					webHandlers.RegisterDomain,
					web.CSRFMiddleware(sessionManager),
					web.RequireAuth(sessionManager),
					web.SecurityHeadersMiddleware,
					web.RequestSizeLimitMiddleware(int64(Config.Security.MaxRequestBodySize)),
					web.LoggingMiddleware,
				))
				api.GET("/dashboard/domain/:username/credentials", web.ChainMiddleware(
					webHandlers.ViewDomainCredentials,
					web.RequireAuth(sessionManager),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.DELETE("/dashboard/domain/:username", web.ChainMiddleware(
					webHandlers.DeleteDomain,
					web.CSRFMiddleware(sessionManager),
					web.RequireAuth(sessionManager),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.POST("/dashboard/domain/:username/description", web.ChainMiddleware(
					webHandlers.UpdateDomainDescription,
					web.CSRFMiddleware(sessionManager),
					web.RequireAuth(sessionManager),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))

				// Profile routes
				api.GET("/profile", web.ChainMiddleware(
					webHandlers.ProfilePage,
					web.RequireAuth(sessionManager),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.POST("/profile/password", web.ChainMiddleware(
					webHandlers.ChangePassword,
					web.CSRFMiddleware(sessionManager),
					web.RequireAuth(sessionManager),
					web.SecurityHeadersMiddleware,
					web.RequestSizeLimitMiddleware(int64(Config.Security.MaxRequestBodySize)),
					web.LoggingMiddleware,
				))
				api.DELETE("/profile/sessions/:id", web.ChainMiddleware(
					webHandlers.RevokeSession,
					web.CSRFMiddleware(sessionManager),
					web.RequireAuth(sessionManager),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))

				// Registration routes (if self-registration enabled)
				// Note: Using /signup for user registration to avoid conflict with API /register endpoint
				if Config.WebUI.AllowSelfRegistration {
					api.GET("/signup", web.ChainMiddleware(
						webHandlers.RegisterPage,
						web.SecurityHeadersMiddleware,
						web.LoggingMiddleware,
					))
					api.POST("/signup", web.ChainMiddleware(
						webHandlers.RegisterPost,
						web.SecurityHeadersMiddleware,
						web.RequestSizeLimitMiddleware(int64(Config.Security.MaxRequestBodySize)),
						web.RateLimitMiddleware(webRateLimiter, Config.Security.RateLimiting),
						web.LoggingMiddleware,
					))
				}

				// Password reset routes (always available)
				api.GET("/password-reset", web.ChainMiddleware(
					webHandlers.PasswordResetRequestPage,
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.POST("/password-reset", web.ChainMiddleware(
					webHandlers.PasswordResetRequestPost,
					web.SecurityHeadersMiddleware,
					web.RequestSizeLimitMiddleware(int64(Config.Security.MaxRequestBodySize)),
					web.RateLimitMiddleware(webRateLimiter, Config.Security.RateLimiting),
					web.LoggingMiddleware,
				))
				api.GET("/password-reset/:token", web.ChainMiddleware(
					webHandlers.PasswordResetPage,
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.POST("/password-reset/:token", web.ChainMiddleware(
					webHandlers.PasswordResetPost,
					web.SecurityHeadersMiddleware,
					web.RequestSizeLimitMiddleware(int64(Config.Security.MaxRequestBodySize)),
					web.RateLimitMiddleware(webRateLimiter, Config.Security.RateLimiting),
					web.LoggingMiddleware,
				))

				// Admin routes (admin authentication required)
				api.GET("/admin", web.ChainMiddleware(
					adminHandlers.Dashboard,
					web.RequireAdmin(sessionManager, userRepo),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.POST("/admin/users", web.ChainMiddleware(
					adminHandlers.CreateUser,
					web.CSRFMiddleware(sessionManager),
					web.RequireAdmin(sessionManager, userRepo),
					web.SecurityHeadersMiddleware,
					web.RequestSizeLimitMiddleware(int64(Config.Security.MaxRequestBodySize)),
					web.LoggingMiddleware,
				))
				api.DELETE("/admin/users/:id", web.ChainMiddleware(
					adminHandlers.DeleteUser,
					web.CSRFMiddleware(sessionManager),
					web.RequireAdmin(sessionManager, userRepo),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.POST("/admin/users/:id/toggle", web.ChainMiddleware(
					adminHandlers.ToggleUserActive,
					web.CSRFMiddleware(sessionManager),
					web.RequireAdmin(sessionManager, userRepo),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.DELETE("/admin/domains/:username", web.ChainMiddleware(
					adminHandlers.DeleteDomain,
					web.CSRFMiddleware(sessionManager),
					web.RequireAdmin(sessionManager, userRepo),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				api.POST("/admin/claim/:username", web.ChainMiddleware(
					adminHandlers.ClaimDomain,
					web.CSRFMiddleware(sessionManager),
					web.RequireAdmin(sessionManager, userRepo),
					web.SecurityHeadersMiddleware,
					web.LoggingMiddleware,
				))
				// Bulk operations
				api.POST("/admin/domains/bulk-claim", web.ChainMiddleware(
					adminHandlers.BulkClaimDomains,
					web.CSRFMiddleware(sessionManager),
					web.RequireAdmin(sessionManager, userRepo),
					web.SecurityHeadersMiddleware,
					web.RequestSizeLimitMiddleware(int64(Config.Security.MaxRequestBodySize)),
					web.LoggingMiddleware,
				))
				api.POST("/admin/domains/bulk-delete", web.ChainMiddleware(
					adminHandlers.BulkDeleteDomains,
					web.CSRFMiddleware(sessionManager),
					web.RequireAdmin(sessionManager, userRepo),
					web.SecurityHeadersMiddleware,
					web.RequestSizeLimitMiddleware(int64(Config.Security.MaxRequestBodySize)),
					web.LoggingMiddleware,
				))

				log.Info("Web UI routes registered successfully")
			}
		}
	}

	host := Config.API.IP + ":" + Config.API.Port

	// TLS specific general settings
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	provider := NewChallengeProvider(dnsservers)
	storage := certmagic.FileStorage{Path: Config.API.ACMECacheDir}

	// Set up certmagic for getting certificate for acme-dns api
	certmagic.DefaultACME.DNS01Solver = &provider
	certmagic.DefaultACME.Agreed = true
	if Config.API.TLS == "letsencrypt" {
		certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
	} else {
		certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
	}
	certmagic.DefaultACME.Email = Config.API.NotificationEmail
	magicConf := certmagic.NewDefault()
	magicConf.Storage = &storage
	magicConf.DefaultServerName = Config.General.Domain

	magicCache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(cert certmagic.Certificate) (*certmagic.Config, error) {
			return magicConf, nil
		},
	})

	magic := certmagic.New(magicCache, *magicConf)
	var err error
	switch Config.API.TLS {
	case "letsencryptstaging":
		err = magic.ManageAsync(context.Background(), []string{Config.General.Domain})
		if err != nil {
			errChan <- err
			return
		}
		cfg.GetCertificate = magic.GetCertificate

		srv := &http.Server{
			Addr:      host,
			Handler:   c.Handler(api),
			TLSConfig: cfg,
			ErrorLog:  stdlog.New(logwriter, "", 0),
		}
		log.WithFields(log.Fields{"host": host, "domain": Config.General.Domain}).Info("Listening HTTPS")
		err = srv.ListenAndServeTLS("", "")
	case "letsencrypt":
		err = magic.ManageAsync(context.Background(), []string{Config.General.Domain})
		if err != nil {
			errChan <- err
			return
		}
		cfg.GetCertificate = magic.GetCertificate
		srv := &http.Server{
			Addr:      host,
			Handler:   c.Handler(api),
			TLSConfig: cfg,
			ErrorLog:  stdlog.New(logwriter, "", 0),
		}
		log.WithFields(log.Fields{"host": host, "domain": Config.General.Domain}).Info("Listening HTTPS")
		err = srv.ListenAndServeTLS("", "")
	case "cert":
		srv := &http.Server{
			Addr:      host,
			Handler:   c.Handler(api),
			TLSConfig: cfg,
			ErrorLog:  stdlog.New(logwriter, "", 0),
		}
		log.WithFields(log.Fields{"host": host}).Info("Listening HTTPS")
		err = srv.ListenAndServeTLS(Config.API.TLSCertFullchain, Config.API.TLSCertPrivkey)
	default:
		log.WithFields(log.Fields{"host": host}).Info("Listening HTTP")
		err = http.ListenAndServe(host, c.Handler(api))
	}
	if err != nil {
		errChan <- err
	}
}
