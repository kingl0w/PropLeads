package database

import (
	"database/sql"
	"log"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() error {
	dbPath := filepath.Join("data", "propleads.db")

	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	// Create users table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		subscription_status TEXT DEFAULT 'free',
		subscription_tier TEXT DEFAULT 'basic',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login DATETIME,
		is_active BOOLEAN DEFAULT 1
	);

	CREATE INDEX IF NOT EXISTS idx_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_username ON users(username);
	`

	_, err = DB.Exec(createTableSQL)
	if err != nil {
		return err
	}

	log.Println("✅ Database initialized successfully")
	return nil
}

func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}
