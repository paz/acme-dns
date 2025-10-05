package main

import (
	"time"

	log "github.com/sirupsen/logrus"
)

// handleDBUpgradeTo2 upgrades the database from version 1 to version 2
// This migration adds support for web UI with user accounts and sessions
func (d *acmedb) handleDBUpgradeTo2() error {
	var err error
	log.Info("Starting database migration from version 1 to version 2")

	tx, err := d.DB.Begin()
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("Error starting transaction for DB upgrade")
		return err
	}

	// Rollback if errored, commit if not
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			log.Error("Database migration rolled back due to error")
			return
		}
		_ = tx.Commit()
		log.Info("Database migration to version 2 completed successfully")
	}()

	// Create users table
	var usersTable string
	if Config.Database.Engine == "sqlite3" {
		usersTable = `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL,
			last_login INTEGER,
			active BOOLEAN NOT NULL DEFAULT 1
		);`
	} else {
		// PostgreSQL
		usersTable = `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT FALSE,
			created_at BIGINT NOT NULL,
			last_login BIGINT,
			active BOOLEAN NOT NULL DEFAULT TRUE
		);`
	}

	_, err = tx.Exec(usersTable)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("Error creating users table")
		return err
	}
	log.Debug("Created users table")

	// Create sessions table
	var sessionsTable string
	if Config.Database.Engine == "sqlite3" {
		sessionsTable = `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			ip_address TEXT,
			user_agent TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`
	} else {
		// PostgreSQL
		sessionsTable = `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id BIGINT NOT NULL,
			created_at BIGINT NOT NULL,
			expires_at BIGINT NOT NULL,
			ip_address TEXT,
			user_agent TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);`
	}

	_, err = tx.Exec(sessionsTable)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("Error creating sessions table")
		return err
	}
	log.Debug("Created sessions table")

	// Add columns to records table
	// SQLite doesn't support adding columns with FOREIGN KEY in ALTER TABLE
	// PostgreSQL supports it
	var alterRecords []string

	if Config.Database.Engine == "sqlite3" {
		alterRecords = []string{
			"ALTER TABLE records ADD COLUMN user_id INTEGER",
			"ALTER TABLE records ADD COLUMN created_at INTEGER",
			"ALTER TABLE records ADD COLUMN description TEXT",
		}
	} else {
		// PostgreSQL
		alterRecords = []string{
			"ALTER TABLE records ADD COLUMN IF NOT EXISTS user_id BIGINT",
			"ALTER TABLE records ADD COLUMN IF NOT EXISTS created_at BIGINT",
			"ALTER TABLE records ADD COLUMN IF NOT EXISTS description TEXT",
		}
	}

	for _, query := range alterRecords {
		_, err = tx.Exec(query)
		if err != nil {
			log.WithFields(log.Fields{"error": err.Error(), "query": query}).Error("Error altering records table")
			return err
		}
	}
	log.Debug("Extended records table with user_id, created_at, and description columns")

	// Set created_at for existing records to current time (use parameterized query)
	now := time.Now().Unix()
	updateSQL := "UPDATE records SET created_at = ? WHERE created_at IS NULL"
	if Config.Database.Engine == "postgres" {
		updateSQL = "UPDATE records SET created_at = $1 WHERE created_at IS NULL"
	}
	_, err = tx.Exec(updateSQL, now)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("Error updating created_at for existing records")
		return err
	}

	// Create indexes for better performance
	var indexes []string
	if Config.Database.Engine == "sqlite3" {
		indexes = []string{
			"CREATE INDEX IF NOT EXISTS idx_txt_subdomain ON txt(Subdomain)",
			"CREATE INDEX IF NOT EXISTS idx_txt_lastupdate ON txt(LastUpdate)",
			"CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)",
			"CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)",
			"CREATE INDEX IF NOT EXISTS idx_records_user_id ON records(user_id)",
		}
	} else {
		// PostgreSQL
		indexes = []string{
			"CREATE INDEX IF NOT EXISTS idx_txt_subdomain ON txt(Subdomain)",
			"CREATE INDEX IF NOT EXISTS idx_txt_lastupdate ON txt(LastUpdate)",
			"CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)",
			"CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)",
			"CREATE INDEX IF NOT EXISTS idx_records_user_id ON records(user_id)",
		}
	}

	for _, index := range indexes {
		_, err = tx.Exec(index)
		if err != nil {
			log.WithFields(log.Fields{"error": err.Error(), "index": index}).Error("Error creating index")
			return err
		}
	}
	log.Debug("Created database indexes for performance optimization")

	// Update database version to 2
	_, err = tx.Exec("UPDATE acmedns SET Value='2' WHERE Name='db_version'")
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("Error updating database version")
		return err
	}

	return nil
}

// CleanupExpiredSessions removes expired sessions from the database
// This should be called periodically (e.g., via a background goroutine)
func (d *acmedb) CleanupExpiredSessions() error {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	now := time.Now().Unix()
	deleteSQL := "DELETE FROM sessions WHERE expires_at < $1"
	if Config.Database.Engine == "sqlite3" {
		deleteSQL = getSQLiteStmt(deleteSQL)
	}

	result, err := d.DB.Exec(deleteSQL, now)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("Error cleaning up expired sessions")
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.WithFields(log.Fields{"count": rowsAffected}).Debug("Cleaned up expired sessions")
	}

	return nil
}

// GetDatabaseStats returns statistics about the database
func (d *acmedb) GetDatabaseStats() (map[string]interface{}, error) {
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	stats := make(map[string]interface{})

	// Count users
	var userCount int
	err := d.DB.QueryRow("SELECT COUNT(*) FROM users WHERE active = TRUE OR active = 1").Scan(&userCount)
	if err != nil {
		userCount = 0
	}
	stats["active_users"] = userCount

	// Count total records
	var recordCount int
	err = d.DB.QueryRow("SELECT COUNT(*) FROM records").Scan(&recordCount)
	if err != nil {
		recordCount = 0
	}
	stats["total_records"] = recordCount

	// Count records with user_id (managed via web UI)
	var managedCount int
	err = d.DB.QueryRow("SELECT COUNT(*) FROM records WHERE user_id IS NOT NULL").Scan(&managedCount)
	if err != nil {
		managedCount = 0
	}
	stats["managed_records"] = managedCount

	// Count unmanaged records (API-only)
	stats["unmanaged_records"] = recordCount - managedCount

	// Count active sessions
	now := time.Now().Unix()
	var sessionCount int
	countSQL := "SELECT COUNT(*) FROM sessions WHERE expires_at > $1"
	if Config.Database.Engine == "sqlite3" {
		countSQL = getSQLiteStmt(countSQL)
	}
	err = d.DB.QueryRow(countSQL, now).Scan(&sessionCount)
	if err != nil {
		sessionCount = 0
	}
	stats["active_sessions"] = sessionCount

	return stats, nil
}
