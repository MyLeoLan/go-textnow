package mysql

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql" // mysql
)

// DB MySQL
type DB struct {
	*sql.DB
}

// NewDB created and returns DB connection to MySQL
func NewDB(connStr string) (*DB, error) {
	// connect to mysql database
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		// Failed to connect to db
		return nil, err
	}

	// check the connection
	err = db.Ping()
	if err != nil {
		// Failed to ping db
		return nil, err
	}

	return &DB{db}, nil
}
