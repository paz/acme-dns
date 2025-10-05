package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
)

// PasswordReset represents a password reset token
type PasswordReset struct {
	Token     string
	UserID    int64
	Email     string
	CreatedAt time.Time
	ExpiresAt time.Time
	Used      bool
}

// PasswordResetRepository handles password reset operations
type PasswordResetRepository struct {
	db *sql.DB
}

// NewPasswordResetRepository creates a new password reset repository
func NewPasswordResetRepository(db *sql.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

// Create generates a new password reset token
func (r *PasswordResetRepository) Create(userID int64, email string, validHours int) (*PasswordReset, error) {
	// Generate secure random token (32 bytes = 44 chars base64)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	now := time.Now()
	expiresAt := now.Add(time.Duration(validHours) * time.Hour)

	// Insert into database
	query := `
		INSERT INTO password_resets (token, user_id, email, created_at, expires_at, used)
		VALUES (?, ?, ?, ?, ?, ?)`

	_, err := r.db.Exec(query, token, userID, email, now.Unix(), expiresAt.Unix(), false)
	if err != nil {
		return nil, fmt.Errorf("failed to create password reset: %w", err)
	}

	return &PasswordReset{
		Token:     token,
		UserID:    userID,
		Email:     email,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Used:      false,
	}, nil
}

// Get retrieves a password reset token
func (r *PasswordResetRepository) Get(token string) (*PasswordReset, error) {
	query := `
		SELECT token, user_id, email, created_at, expires_at, used
		FROM password_resets
		WHERE token = ?`

	var pr PasswordReset
	var createdAt, expiresAt int64

	err := r.db.QueryRow(query, token).Scan(
		&pr.Token,
		&pr.UserID,
		&pr.Email,
		&createdAt,
		&expiresAt,
		&pr.Used,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("token not found")
		}
		return nil, fmt.Errorf("failed to get password reset: %w", err)
	}

	pr.CreatedAt = time.Unix(createdAt, 0)
	pr.ExpiresAt = time.Unix(expiresAt, 0)

	return &pr, nil
}

// GetValid retrieves and validates a password reset token
func (r *PasswordResetRepository) GetValid(token string) (*PasswordReset, error) {
	pr, err := r.Get(token)
	if err != nil {
		return nil, err
	}

	// Check if token is expired
	if time.Now().After(pr.ExpiresAt) {
		return nil, fmt.Errorf("token has expired")
	}

	// Check if token has been used
	if pr.Used {
		return nil, fmt.Errorf("token has already been used")
	}

	return pr, nil
}

// MarkUsed marks a password reset token as used
func (r *PasswordResetRepository) MarkUsed(token string) error {
	query := `UPDATE password_resets SET used = ? WHERE token = ?`

	result, err := r.db.Exec(query, true, token)
	if err != nil {
		return fmt.Errorf("failed to mark token as used: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("token not found")
	}

	return nil
}

// DeleteExpired removes expired password reset tokens
func (r *PasswordResetRepository) DeleteExpired() error {
	query := `DELETE FROM password_resets WHERE expires_at < ?`

	result, err := r.db.Exec(query, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	rows, err := result.RowsAffected()
	if err == nil && rows > 0 {
		log.WithFields(log.Fields{"count": rows}).Debug("Deleted expired password reset tokens")
	}

	return nil
}

// DeleteByUserID removes all password reset tokens for a user
func (r *PasswordResetRepository) DeleteByUserID(userID int64) error {
	query := `DELETE FROM password_resets WHERE user_id = ?`

	_, err := r.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete tokens for user: %w", err)
	}

	return nil
}

// CreatePasswordResetTable creates the password_resets table if it doesn't exist
func CreatePasswordResetTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS password_resets (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			email TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			used BOOLEAN DEFAULT 0
		)`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create password_resets table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_password_resets_user_id ON password_resets(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_password_resets_expires_at ON password_resets(expires_at)",
	}

	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	log.Info("Password reset table and indexes created successfully")
	return nil
}
