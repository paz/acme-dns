package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
)

// Record represents a DNS record registration
type Record struct {
	Username    string
	Password    string
	Subdomain   string
	AllowFrom   []string
	UserID      *int64
	CreatedAt   *time.Time
	Description *string
}

// RecordRepository handles database operations for records
type RecordRepository struct {
	DB     *sql.DB
	Engine string // "sqlite3" or "postgres"
}

// NewRecordRepository creates a new RecordRepository
func NewRecordRepository(db *sql.DB, engine string) *RecordRepository {
	return &RecordRepository{
		DB:     db,
		Engine: engine,
	}
}

// getSQLiteStmt replaces PostgreSQL placeholders with SQLite variant
func (rr *RecordRepository) getSQLiteStmt(s string) string {
	re, _ := regexp.Compile(`\$[0-9]`)
	return re.ReplaceAllString(s, "?")
}

// GetByUsername retrieves a record by username
func (rr *RecordRepository) GetByUsername(username string) (*Record, error) {
	selectSQL := `
		SELECT Username, Password, Subdomain, AllowFrom, user_id, created_at, description
		FROM records
		WHERE Username = $1
	`
	if rr.Engine == "sqlite3" {
		selectSQL = rr.getSQLiteStmt(selectSQL)
	}

	record := &Record{}
	var allowFromJSON string
	var userID sql.NullInt64
	var createdAt sql.NullInt64
	var description sql.NullString

	err := rr.DB.QueryRow(selectSQL, username).Scan(
		&record.Username,
		&record.Password,
		&record.Subdomain,
		&allowFromJSON,
		&userID,
		&createdAt,
		&description,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("record not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get record: %w", err)
	}

	// Parse AllowFrom JSON
	var allowFrom []string
	if err := json.Unmarshal([]byte(allowFromJSON), &allowFrom); err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("Failed to unmarshal AllowFrom")
		allowFrom = []string{}
	}
	record.AllowFrom = allowFrom

	if userID.Valid {
		uid := userID.Int64
		record.UserID = &uid
	}

	if createdAt.Valid {
		t := time.Unix(createdAt.Int64, 0)
		record.CreatedAt = &t
	}

	if description.Valid {
		record.Description = &description.String
	}

	return record, nil
}

// ListByUserID returns all records for a specific user
func (rr *RecordRepository) ListByUserID(userID int64) ([]*Record, error) {
	selectSQL := `
		SELECT Username, Password, Subdomain, AllowFrom, user_id, created_at, description
		FROM records
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	if rr.Engine == "sqlite3" {
		selectSQL = rr.getSQLiteStmt(selectSQL)
	}

	rows, err := rr.DB.Query(selectSQL, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var records []*Record
	for rows.Next() {
		record := &Record{}
		var allowFromJSON string
		var userIDVal sql.NullInt64
		var createdAt sql.NullInt64
		var description sql.NullString

		err := rows.Scan(
			&record.Username,
			&record.Password,
			&record.Subdomain,
			&allowFromJSON,
			&userIDVal,
			&createdAt,
			&description,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}

		// Parse AllowFrom JSON
		var allowFrom []string
		if err := json.Unmarshal([]byte(allowFromJSON), &allowFrom); err != nil {
			log.WithFields(log.Fields{"error": err.Error()}).Error("Failed to unmarshal AllowFrom")
			allowFrom = []string{}
		}
		record.AllowFrom = allowFrom

		if userIDVal.Valid {
			uid := userIDVal.Int64
			record.UserID = &uid
		}

		if createdAt.Valid {
			t := time.Unix(createdAt.Int64, 0)
			record.CreatedAt = &t
		}

		if description.Valid {
			record.Description = &description.String
		}

		records = append(records, record)
	}

	return records, nil
}

// ListAll returns all records (admin function)
func (rr *RecordRepository) ListAll() ([]*Record, error) {
	selectSQL := `
		SELECT Username, Password, Subdomain, AllowFrom, user_id, created_at, description
		FROM records
		ORDER BY created_at DESC
	`

	rows, err := rr.DB.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to list all records: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var records []*Record
	for rows.Next() {
		record := &Record{}
		var allowFromJSON string
		var userIDVal sql.NullInt64
		var createdAt sql.NullInt64
		var description sql.NullString

		err := rows.Scan(
			&record.Username,
			&record.Password,
			&record.Subdomain,
			&allowFromJSON,
			&userIDVal,
			&createdAt,
			&description,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}

		// Parse AllowFrom JSON
		var allowFrom []string
		if err := json.Unmarshal([]byte(allowFromJSON), &allowFrom); err != nil {
			log.WithFields(log.Fields{"error": err.Error()}).Error("Failed to unmarshal AllowFrom")
			allowFrom = []string{}
		}
		record.AllowFrom = allowFrom

		if userIDVal.Valid {
			uid := userIDVal.Int64
			record.UserID = &uid
		}

		if createdAt.Valid {
			t := time.Unix(createdAt.Int64, 0)
			record.CreatedAt = &t
		}

		if description.Valid {
			record.Description = &description.String
		}

		records = append(records, record)
	}

	return records, nil
}

// ListUnmanaged returns all records without a user_id (API-only registrations)
func (rr *RecordRepository) ListUnmanaged() ([]*Record, error) {
	selectSQL := `
		SELECT Username, Password, Subdomain, AllowFrom, user_id, created_at, description
		FROM records
		WHERE user_id IS NULL
		ORDER BY created_at DESC
	`

	rows, err := rr.DB.Query(selectSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to list unmanaged records: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var records []*Record
	for rows.Next() {
		record := &Record{}
		var allowFromJSON string
		var userIDVal sql.NullInt64
		var createdAt sql.NullInt64
		var description sql.NullString

		err := rows.Scan(
			&record.Username,
			&record.Password,
			&record.Subdomain,
			&allowFromJSON,
			&userIDVal,
			&createdAt,
			&description,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}

		// Parse AllowFrom JSON
		var allowFrom []string
		if err := json.Unmarshal([]byte(allowFromJSON), &allowFrom); err != nil {
			log.WithFields(log.Fields{"error": err.Error()}).Error("Failed to unmarshal AllowFrom")
			allowFrom = []string{}
		}
		record.AllowFrom = allowFrom

		if userIDVal.Valid {
			uid := userIDVal.Int64
			record.UserID = &uid
		}

		if createdAt.Valid {
			t := time.Unix(createdAt.Int64, 0)
			record.CreatedAt = &t
		}

		if description.Valid {
			record.Description = &description.String
		}

		records = append(records, record)
	}

	return records, nil
}

// ClaimRecord associates an unmanaged record with a user
func (rr *RecordRepository) ClaimRecord(username string, userID int64, description string) error {
	updateSQL := "UPDATE records SET user_id = $1, description = $2 WHERE Username = $3 AND user_id IS NULL"
	if rr.Engine == "sqlite3" {
		updateSQL = rr.getSQLiteStmt(updateSQL)
	}

	result, err := rr.DB.Exec(updateSQL, userID, description, username)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "username": username}).Error("Failed to claim record")
		return fmt.Errorf("failed to claim record: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("record not found or already claimed")
	}

	log.WithFields(log.Fields{"username": username, "user_id": userID}).Info("Claimed record")
	return nil
}

// UpdateDescription updates a record's description
func (rr *RecordRepository) UpdateDescription(username string, userID int64, description string) error {
	updateSQL := "UPDATE records SET description = $1 WHERE Username = $2 AND user_id = $3"
	if rr.Engine == "sqlite3" {
		updateSQL = rr.getSQLiteStmt(updateSQL)
	}

	result, err := rr.DB.Exec(updateSQL, description, username, userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "username": username}).Error("Failed to update description")
		return fmt.Errorf("failed to update description: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("record not found or not owned by user")
	}

	return nil
}

// Delete deletes a record
func (rr *RecordRepository) Delete(username string, userID int64) error {
	// First delete associated TXT records
	deleteTxtSQL := "DELETE FROM txt WHERE Subdomain = (SELECT Subdomain FROM records WHERE Username = $1 AND user_id = $2)"
	if rr.Engine == "sqlite3" {
		deleteTxtSQL = rr.getSQLiteStmt(deleteTxtSQL)
	}

	_, err := rr.DB.Exec(deleteTxtSQL, username, userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "username": username}).Error("Failed to delete TXT records")
		return fmt.Errorf("failed to delete TXT records: %w", err)
	}

	// Then delete the record itself
	deleteRecordSQL := "DELETE FROM records WHERE Username = $1 AND user_id = $2"
	if rr.Engine == "sqlite3" {
		deleteRecordSQL = rr.getSQLiteStmt(deleteRecordSQL)
	}

	result, err := rr.DB.Exec(deleteRecordSQL, username, userID)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "username": username}).Error("Failed to delete record")
		return fmt.Errorf("failed to delete record: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("record not found or not owned by user")
	}

	log.WithFields(log.Fields{"username": username, "user_id": userID}).Info("Deleted record")
	return nil
}

// DeleteByAdmin deletes a record (admin function, bypasses user ownership check)
func (rr *RecordRepository) DeleteByAdmin(username string) error {
	// First delete associated TXT records
	deleteTxtSQL := "DELETE FROM txt WHERE Subdomain = (SELECT Subdomain FROM records WHERE Username = $1)"
	if rr.Engine == "sqlite3" {
		deleteTxtSQL = rr.getSQLiteStmt(deleteTxtSQL)
	}

	_, err := rr.DB.Exec(deleteTxtSQL, username)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "username": username}).Error("Failed to delete TXT records")
		return fmt.Errorf("failed to delete TXT records: %w", err)
	}

	// Then delete the record itself
	deleteRecordSQL := "DELETE FROM records WHERE Username = $1"
	if rr.Engine == "sqlite3" {
		deleteRecordSQL = rr.getSQLiteStmt(deleteRecordSQL)
	}

	result, err := rr.DB.Exec(deleteRecordSQL, username)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "username": username}).Error("Failed to delete record")
		return fmt.Errorf("failed to delete record: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("record not found")
	}

	log.WithFields(log.Fields{"username": username}).Info("Admin deleted record")
	return nil
}

// GetTXTRecords retrieves the TXT record values for a subdomain
func (rr *RecordRepository) GetTXTRecords(subdomain string) ([]string, error) {
	selectSQL := "SELECT Value FROM txt WHERE Subdomain = $1 LIMIT 2"
	if rr.Engine == "sqlite3" {
		selectSQL = rr.getSQLiteStmt(selectSQL)
	}

	rows, err := rr.DB.Query(selectSQL, subdomain)
	if err != nil {
		return nil, fmt.Errorf("failed to get TXT records: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("failed to scan TXT record: %w", err)
		}
		values = append(values, value)
	}

	return values, nil
}
