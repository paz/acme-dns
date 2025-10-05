//go:build !test
// +build !test

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/joohoi/acme-dns/models"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// CreateAdminUser creates a new admin user via CLI
func CreateAdminUser(email string) error {
	// Validate email
	if email == "" {
		return fmt.Errorf("email is required")
	}

	// Initialize database
	newDB := new(acmedb)
	err := newDB.Init(Config.Database.Engine, Config.Database.Connection)
	if err != nil {
		return fmt.Errorf("could not open database: %v", err)
	}
	defer newDB.Close()

	// Get password from user (with confirmation)
	password, err := promptPassword()
	if err != nil {
		return err
	}

	// Create user repository
	userRepo := models.NewUserRepository(newDB.GetBackend(), Config.Database.Engine)

	log.WithFields(log.Fields{"email": email}).Info("Creating admin user...")

	// Create admin user with bcrypt cost 12
	user, err := userRepo.Create(email, password, true, BcryptCostWeb)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %v", err)
	}

	fmt.Printf("\n✅ Admin user created successfully!\n")
	fmt.Printf("Email: %s\n", user.Email)
	fmt.Printf("User ID: %d\n", user.ID)
	fmt.Printf("\nYou can now login at: https://your-domain/login\n\n")

	return nil
}

// promptPassword prompts for password with confirmation
func promptPassword() (string, error) {
	fmt.Print("Enter password (min 12 chars): ")
	password, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	fmt.Println()

	if len(password) < DefaultMinPasswordLength {
		return "", fmt.Errorf("password must be at least %d characters", DefaultMinPasswordLength)
	}

	fmt.Print("Confirm password: ")
	confirm, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	fmt.Println()

	if string(password) != string(confirm) {
		return "", fmt.Errorf("passwords do not match")
	}

	return string(password), nil
}

// ShowVersion shows version information
func ShowVersion() {
	fmt.Printf("acme-dns version 2.0.0\n")
	fmt.Printf("Database version: %d\n", CurrentDBVersion)
	fmt.Printf("Go version: %s\n", "1.22+")
}

// ShowDatabaseInfo shows database migration status
func ShowDatabaseInfo() error {
	newDB := new(acmedb)
	err := newDB.Init(Config.Database.Engine, Config.Database.Connection)
	if err != nil {
		return fmt.Errorf("could not open database: %v", err)
	}
	defer newDB.Close()

	var versionString string
	_ = newDB.GetBackend().QueryRow("SELECT Value FROM acmedns WHERE Name='db_version'").Scan(&versionString)

	fmt.Printf("Database Information\n")
	fmt.Printf("====================\n")
	fmt.Printf("Engine: %s\n", Config.Database.Engine)
	fmt.Printf("Current schema version: %s\n", versionString)
	fmt.Printf("Expected schema version: %d\n", CurrentDBVersion)

	if versionString == fmt.Sprintf("%d", CurrentDBVersion) {
		fmt.Printf("Status: ✅ Up to date\n")
	} else {
		fmt.Printf("Status: ⚠️  Migration needed (will run automatically on startup)\n")
	}

	return nil
}

// PromptYesNo prompts for yes/no confirmation
func PromptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (y/n): ", question)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
