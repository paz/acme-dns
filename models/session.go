package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
)

// Session represents a user session
type Session struct {
	ID        string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
	IPAddress string
	UserAgent string
}

// SessionRepository handles database operations for sessions
type SessionRepository struct {
	DB     *sql.DB
	Engine string // "sqlite3" or "postgres"
}

// NewSessionRepository creates a new SessionRepository
func NewSessionRepository(db *sql.DB, engine string) *SessionRepository {
	return &SessionRepository{
		DB:     db,
		Engine: engine,
	}
}

// getSQLiteStmt replaces PostgreSQL placeholders with SQLite variant
func (sr *SessionRepository) getSQLiteStmt(s string) string {
	re, _ := regexp.Compile(`\$[0-9]`)
	return re.ReplaceAllString(s, "?")
}

// GenerateSessionID generates a cryptographically secure random session ID
func GenerateSessionID(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// Create creates a new session
func (sr *SessionRepository) Create(userID int64, durationHours int, ipAddress, userAgent string) (*Session, error) {
	sessionID, err := GenerateSessionID(48) // 48 bytes = 64 chars in base64
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(durationHours) * time.Hour)

	insertSQL := `
		INSERT INTO sessions (id, user_id, created_at, expires_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	if sr.Engine == "sqlite3" {
		insertSQL = sr.getSQLiteStmt(insertSQL)
	}

	_, err = sr.DB.Exec(
		insertSQL,
		sessionID,
		userID,
		now.Unix(),
		expiresAt.Unix(),
		ipAddress,
		userAgent,
	)

	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "user_id": userID}).Error("Failed to create session")
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	log.WithFields(log.Fields{"session_id": sessionID, "user_id": userID}).Debug("Created new session")
	return session, nil
}

// Get retrieves a session by ID
func (sr *SessionRepository) Get(sessionID string) (*Session, error) {
	selectSQL := `
		SELECT id, user_id, created_at, expires_at, ip_address, user_agent
		FROM sessions
		WHERE id = $1
	`
	if sr.Engine == "sqlite3" {
		selectSQL = sr.getSQLiteStmt(selectSQL)
	}

	session := &Session{}
	var createdAt, expiresAt int64

	err := sr.DB.QueryRow(selectSQL, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&createdAt,
		&expiresAt,
		&session.IPAddress,
		&session.UserAgent,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	session.CreatedAt = time.Unix(createdAt, 0)
	session.ExpiresAt = time.Unix(expiresAt, 0)

	return session, nil
}

// GetValid retrieves a session by ID and checks if it's still valid (not expired)
func (sr *SessionRepository) GetValid(sessionID string) (*Session, error) {
	session, err := sr.Get(sessionID)
	if err != nil {
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		// Session expired, delete it
		_ = sr.Delete(sessionID)
		return nil, errors.New("session expired")
	}

	return session, nil
}

// Delete deletes a session
func (sr *SessionRepository) Delete(sessionID string) error {
	deleteSQL := "DELETE FROM sessions WHERE id = $1"
	if sr.Engine == "sqlite3" {
		deleteSQL = sr.getSQLiteStmt(deleteSQL)
	}

	_, err := sr.DB.Exec(deleteSQL, sessionID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "session_id": sessionID}).Error("Failed to delete session")
		return fmt.Errorf("failed to delete session: %w", err)
	}

	log.WithFields(log.Fields{"session_id": sessionID}).Debug("Deleted session")
	return nil
}

// DeleteByUserID deletes all sessions for a specific user
func (sr *SessionRepository) DeleteByUserID(userID int64) error {
	deleteSQL := "DELETE FROM sessions WHERE user_id = $1"
	if sr.Engine == "sqlite3" {
		deleteSQL = sr.getSQLiteStmt(deleteSQL)
	}

	result, err := sr.DB.Exec(deleteSQL, userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "user_id": userID}).Error("Failed to delete user sessions")
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.WithFields(log.Fields{"user_id": userID, "count": rowsAffected}).Debug("Deleted user sessions")
	return nil
}

// DeleteExpired deletes all expired sessions
func (sr *SessionRepository) DeleteExpired() error {
	now := time.Now().Unix()
	deleteSQL := "DELETE FROM sessions WHERE expires_at < $1"
	if sr.Engine == "sqlite3" {
		deleteSQL = sr.getSQLiteStmt(deleteSQL)
	}

	result, err := sr.DB.Exec(deleteSQL, now)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("Failed to delete expired sessions")
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.WithFields(log.Fields{"count": rowsAffected}).Debug("Deleted expired sessions")
	}
	return nil
}

// Extend extends a session's expiration time
func (sr *SessionRepository) Extend(sessionID string, additionalHours int) error {
	session, err := sr.GetValid(sessionID)
	if err != nil {
		return err
	}

	newExpiresAt := time.Now().Add(time.Duration(additionalHours) * time.Hour)

	updateSQL := "UPDATE sessions SET expires_at = $1 WHERE id = $2"
	if sr.Engine == "sqlite3" {
		updateSQL = sr.getSQLiteStmt(updateSQL)
	}

	_, err = sr.DB.Exec(updateSQL, newExpiresAt.Unix(), sessionID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "session_id": sessionID}).Error("Failed to extend session")
		return fmt.Errorf("failed to extend session: %w", err)
	}

	log.WithFields(log.Fields{"session_id": sessionID, "user_id": session.UserID}).Debug("Extended session")
	return nil
}

// ListByUserID returns all active sessions for a user
func (sr *SessionRepository) ListByUserID(userID int64) ([]*Session, error) {
	now := time.Now().Unix()
	selectSQL := `
		SELECT id, user_id, created_at, expires_at, ip_address, user_agent
		FROM sessions
		WHERE user_id = $1 AND expires_at > $2
		ORDER BY created_at DESC
	`
	if sr.Engine == "sqlite3" {
		selectSQL = sr.getSQLiteStmt(selectSQL)
	}

	rows, err := sr.DB.Query(selectSQL, userID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var sessions []*Session
	for rows.Next() {
		session := &Session{}
		var createdAt, expiresAt int64

		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&createdAt,
			&expiresAt,
			&session.IPAddress,
			&session.UserAgent,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		session.CreatedAt = time.Unix(createdAt, 0)
		session.ExpiresAt = time.Unix(expiresAt, 0)

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// Count returns the number of active sessions
func (sr *SessionRepository) Count() (int, error) {
	now := time.Now().Unix()
	countSQL := "SELECT COUNT(*) FROM sessions WHERE expires_at > $1"
	if sr.Engine == "sqlite3" {
		countSQL = sr.getSQLiteStmt(countSQL)
	}

	var count int
	err := sr.DB.QueryRow(countSQL, now).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}

	return count, nil
}
