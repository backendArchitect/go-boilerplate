package database

import (
	"database/sql"
	"os"
	"strconv"

	"github.com/codeArtisanry/go-boilerplate/config"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql" // import mysql if it is used
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	_ "github.com/mattn/go-sqlite3"
)

var dbURL string
var err error
var DB *sql.DB

const (
	POSTGRES = "postgres"
	MYSQL    = "mysql"
	SQLITE3  = "sqlite3"
)

// Database interface for common database operations
type Database interface {
	Connect(cfg config.DBConfig) (*sql.DB, error)
	Close() error
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type DBConn struct {
	DatabaseConn Database
}

func NewDBConn(databaseConn DBConn) *DBConn {
	return &DBConn{
		DatabaseConn: databaseConn.DatabaseConn,
	}
}

// Postgres implicitly implements Database
var _ Database = &Postgres{}

// MySQL implicitly implements Database
var _ Database = &MySQL{}

// SQLite3 implicitly implements Database
var _ Database = &SQLite3{}

// / Create me a structs for each database for implementation of interfaces later
type Postgres struct {
	DB *sql.DB
}

type MySQL struct {
	DB *sql.DB
}

type SQLite3 struct {
	DB *sql.DB
}

// Create me a function for mysql database of namme Connect to connect to the database
func (m *MySQL) Connect(cfg config.DBConfig) (*sql.DB, error) {
	dbURL = cfg.Username + ":" + cfg.Password + "@tcp(" + cfg.Host + ":" + strconv.Itoa(cfg.Port) + ")/" + cfg.Db + "?" + cfg.QueryString
	if m.DB == nil {
		m.DB, err = sql.Open(MYSQL, dbURL)
		if err != nil {
			return nil, err
		}
		return m.DB, err
	}
	return m.DB, err
}

// Create me a function for postgres database of namme Connect to connect to the database
func (p *Postgres) Connect(cfg config.DBConfig) (*sql.DB, error) {
	dbURL = "postgres://" + cfg.Username + ":" + cfg.Password + "@" + cfg.Host + ":" + strconv.Itoa(cfg.Port) + "/" + cfg.Db + "?" + cfg.QueryString
	if p.DB == nil {
		p.DB, err = sql.Open(POSTGRES, dbURL)
		if err != nil {
			return nil, err
		}
		return p.DB, err
	}
	return p.DB, err
}

// Create me a function for sqlite3 database of namme Connect to connect to the database
func (s *SQLite3) Connect(cfg config.DBConfig) (*sql.DB, error) {

	if _, err = os.Stat(cfg.SQLiteFilePath); err != nil {
		file, err := os.Create(cfg.SQLiteFilePath)
		if err != nil {
			panic(err)
		}
		err = file.Close()
		if err != nil {
			return nil, err
		}
	}

	if s.DB == nil {
		s.DB, err = sql.Open(SQLITE3, "./"+cfg.SQLiteFilePath)
		if err != nil {
			return nil, err
		}
		return s.DB, err
	}
	return s.DB, err
}

// Create me a function for mysql database of name Close to close the database
func (m *MySQL) Close() error {
	if m.DB != nil {
		err = m.DB.Close()
		if err != nil {
			return err
		}
	}
	return err
}

// Create me a function for postgres database of name Close to close the database
func (p *Postgres) Close() error {
	if p.DB != nil {
		err = p.DB.Close()
		if err != nil {
			return err
		}
	}
	return err
}

// Create me a function for sqlite3 database of name Close to close the database
func (s *SQLite3) Close() error {
	if s.DB != nil {
		err = s.DB.Close()
		if err != nil {
			return err
		}
	}
	return err
}

// Create me a function for mysql database of name query to query the database
func (m *MySQL) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return m.DB.Query(query, args...)
}

// Create me a function for postgres database of name query to query the database
func (p *Postgres) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return p.DB.Query(query, args...)
}

// Create me a function for sqlite3 database of name query to query the database
func (s *SQLite3) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return s.DB.Query(query, args...)
}
