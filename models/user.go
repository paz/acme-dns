package models

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// User represents a web UI user account
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	IsAdmin      bool
	CreatedAt    time.Time
	LastLogin    *time.Time
	Active       bool
}

// UserRepository handles database operations for users
type UserRepository struct {
	DB     *sql.DB
	Engine string // "sqlite3" or "postgres"
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *sql.DB, engine string) *UserRepository {
	return &UserRepository{
		DB:     db,
		Engine: engine,
	}
}

// getSQLiteStmt replaces PostgreSQL placeholders with SQLite variant
func (ur *UserRepository) getSQLiteStmt(s string) string {
	re, _ := regexp.Compile(`\$[0-9]`)
	return re.ReplaceAllString(s, "?")
}

// ValidateEmail checks if email format is valid
func ValidateEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// ValidatePassword checks if password meets security requirements
func ValidatePassword(password string, minLength int) error {
	if len(password) < minLength {
		return fmt.Errorf("password must be at least %d characters long", minLength)
	}

	// Check for at least one uppercase letter
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	// Check for at least one lowercase letter
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	// Check for at least one digit
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)

	if !hasUpper || !hasLower || !hasDigit {
		return errors.New("password must contain at least one uppercase letter, one lowercase letter, and one digit")
	}

	return nil
}

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword compares password with hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Create creates a new user in the database
func (ur *UserRepository) Create(email, password string, isAdmin bool, bcryptCost int) (*User, error) {
	// Validate email
	email = strings.TrimSpace(strings.ToLower(email))
	if !ValidateEmail(email) {
		return nil, errors.New("invalid email format")
	}

	// Check if user already exists
	exists, err := ur.EmailExists(email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("user with this email already exists")
	}

	// Hash password
	passwordHash, err := HashPassword(password, bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert user
	insertSQL := `
		INSERT INTO users (email, password_hash, is_admin, created_at, active)
		VALUES ($1, $2, $3, $4, $5)
	`
	if ur.Engine == "sqlite3" {
		insertSQL = ur.getSQLiteStmt(insertSQL)
		insertSQL += " RETURNING id"
	} else {
		insertSQL += " RETURNING id"
	}

	now := time.Now().Unix()
	var userID int64

	if ur.Engine == "sqlite3" {
		err = ur.DB.QueryRow(insertSQL, email, passwordHash, isAdmin, now, true).Scan(&userID)
	} else {
		err = ur.DB.QueryRow(insertSQL, email, passwordHash, isAdmin, now, true).Scan(&userID)
	}

	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "email": email}).Error("Failed to create user")
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	user := &User{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
		IsAdmin:      isAdmin,
		CreatedAt:    time.Unix(now, 0),
		Active:       true,
	}

	log.WithFields(log.Fields{"user_id": userID, "email": email, "is_admin": isAdmin}).Info("Created new user")
	return user, nil
}

// GetByID retrieves a user by ID
func (ur *UserRepository) GetByID(id int64) (*User, error) {
	selectSQL := `
		SELECT id, email, password_hash, is_admin, created_at, last_login, active
		FROM users
		WHERE id = $1
	`
	if ur.Engine == "sqlite3" {
		selectSQL = ur.getSQLiteStmt(selectSQL)
	}

	user := &User{}
	var createdAt int64
	var lastLogin sql.NullInt64

	err := ur.DB.QueryRow(selectSQL, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.IsAdmin,
		&createdAt,
		&lastLogin,
		&user.Active,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.CreatedAt = time.Unix(createdAt, 0)
	if lastLogin.Valid {
		t := time.Unix(lastLogin.Int64, 0)
		user.LastLogin = &t
	}

	return user, nil
}

// GetByEmail retrieves a user by email address
func (ur *UserRepository) GetByEmail(email string) (*User, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	selectSQL := `
		SELECT id, email, password_hash, is_admin, created_at, last_login, active
		FROM users
		WHERE email = $1
	`
	if ur.Engine == "sqlite3" {
		selectSQL = ur.getSQLiteStmt(selectSQL)
	}

	user := &User{}
	var createdAt int64
	var lastLogin sql.NullInt64

	err := ur.DB.QueryRow(selectSQL, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.IsAdmin,
		&createdAt,
		&lastLogin,
		&user.Active,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.CreatedAt = time.Unix(createdAt, 0)
	if lastLogin.Valid {
		t := time.Unix(lastLogin.Int64, 0)
		user.LastLogin = &t
	}

	return user, nil
}

// EmailExists checks if a user with the given email exists
func (ur *UserRepository) EmailExists(email string) (bool, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	countSQL := "SELECT COUNT(*) FROM users WHERE email = $1"
	if ur.Engine == "sqlite3" {
		countSQL = ur.getSQLiteStmt(countSQL)
	}

	var count int
	err := ur.DB.QueryRow(countSQL, email).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// UpdateLastLogin updates the last login timestamp for a user
func (ur *UserRepository) UpdateLastLogin(userID int64) error {
	updateSQL := "UPDATE users SET last_login = $1 WHERE id = $2"
	if ur.Engine == "sqlite3" {
		updateSQL = ur.getSQLiteStmt(updateSQL)
	}

	now := time.Now().Unix()
	_, err := ur.DB.Exec(updateSQL, now, userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "user_id": userID}).Error("Failed to update last login")
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// ChangePassword changes a user's password
func (ur *UserRepository) ChangePassword(userID int64, newPassword string, bcryptCost int) error {
	// Hash new password
	passwordHash, err := HashPassword(newPassword, bcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	updateSQL := "UPDATE users SET password_hash = $1 WHERE id = $2"
	if ur.Engine == "sqlite3" {
		updateSQL = ur.getSQLiteStmt(updateSQL)
	}

	_, err = ur.DB.Exec(updateSQL, passwordHash, userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "user_id": userID}).Error("Failed to change password")
		return fmt.Errorf("failed to change password: %w", err)
	}

	log.WithFields(log.Fields{"user_id": userID}).Info("User password changed")
	return nil
}

// UpdateEmail updates a user's email address
func (ur *UserRepository) UpdateEmail(userID int64, newEmail string) error {
	newEmail = strings.TrimSpace(strings.ToLower(newEmail))

	if !ValidateEmail(newEmail) {
		return errors.New("invalid email format")
	}

	// Check if email is already in use
	exists, err := ur.EmailExists(newEmail)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("email already in use")
	}

	updateSQL := "UPDATE users SET email = $1 WHERE id = $2"
	if ur.Engine == "sqlite3" {
		updateSQL = ur.getSQLiteStmt(updateSQL)
	}

	_, err = ur.DB.Exec(updateSQL, newEmail, userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "user_id": userID}).Error("Failed to update email")
		return fmt.Errorf("failed to update email: %w", err)
	}

	log.WithFields(log.Fields{"user_id": userID, "new_email": newEmail}).Info("User email updated")
	return nil
}

// SetActive sets a user's active status
func (ur *UserRepository) SetActive(userID int64, active bool) error {
	updateSQL := "UPDATE users SET active = $1 WHERE id = $2"
	if ur.Engine == "sqlite3" {
		updateSQL = ur.getSQLiteStmt(updateSQL)
	}

	_, err := ur.DB.Exec(updateSQL, active, userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "user_id": userID}).Error("Failed to set active status")
		return fmt.Errorf("failed to set active status: %w", err)
	}

	log.WithFields(log.Fields{"user_id": userID, "active": active}).Info("User active status changed")
	return nil
}

// Delete deletes a user (soft delete by setting active = false)
func (ur *UserRepository) Delete(userID int64) error {
	return ur.SetActive(userID, false)
}

// ListAll returns all users
func (ur *UserRepository) ListAll(activeOnly bool) ([]*User, error) {
	var selectSQL string
	if activeOnly {
		selectSQL = `
			SELECT id, email, password_hash, is_admin, created_at, last_login, active
			FROM users
			WHERE active = TRUE OR active = 1
			ORDER BY created_at DESC
		`
	} else {
		selectSQL = `
			SELECT id, email, password_hash, is_admin, created_at, last_login, active
			FROM users
			ORDER BY created_at DESC
		`
	}

	rows, err := ur.DB.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		var createdAt int64
		var lastLogin sql.NullInt64

		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.PasswordHash,
			&user.IsAdmin,
			&createdAt,
			&lastLogin,
			&user.Active,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		user.CreatedAt = time.Unix(createdAt, 0)
		if lastLogin.Valid {
			t := time.Unix(lastLogin.Int64, 0)
			user.LastLogin = &t
		}

		users = append(users, user)
	}

	return users, nil
}

// Authenticate verifies email and password, returns user if successful
func (ur *UserRepository) Authenticate(email, password string) (*User, error) {
	user, err := ur.GetByEmail(email)
	if err != nil {
		// Return generic error to avoid user enumeration
		return nil, errors.New("invalid credentials")
	}

	if !user.Active {
		return nil, errors.New("account is disabled")
	}

	if !VerifyPassword(password, user.PasswordHash) {
		return nil, errors.New("invalid credentials")
	}

	// Update last login
	_ = ur.UpdateLastLogin(user.ID)

	return user, nil
}
