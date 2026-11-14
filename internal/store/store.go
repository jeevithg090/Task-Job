// Package store persists users in SQLite.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/mattn/go-sqlite3"
)

// Errors returned by the store.
var (
	ErrNotFound   = errors.New("user not found")
	ErrEmailTaken = errors.New("email already registered")
)

// User is a stored account. PasswordHash is never serialised.
type User struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	PasswordHash string `json:"-"`
}

// Store provides access to the users table.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	name     TEXT NOT NULL,
	email    TEXT UNIQUE NOT NULL COLLATE NOCASE,
	password TEXT NOT NULL,
	role     TEXT NOT NULL
);`

// Open opens (creating if necessary) the SQLite database at path and applies the schema.
func Open(ctx context.Context, path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error { return s.db.Close() }

// Ping verifies the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// CreateUser inserts a user and returns it with its new ID.
func (s *Store) CreateUser(ctx context.Context, name, email, passwordHash, role string) (User, error) {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO users (name, email, password, role) VALUES (?, ?, ?, ?)",
		name, email, passwordHash, role,
	)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("read user id: %w", err)
	}
	return User{ID: id, Name: name, Email: email, Role: role, PasswordHash: passwordHash}, nil
}

// UserByEmail looks a user up by email, case-insensitively.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.scanOne(ctx,
		"SELECT id, name, email, password, role FROM users WHERE email = ? COLLATE NOCASE",
		strings.TrimSpace(email),
	)
}

// UserByID looks a user up by primary key.
func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return s.scanOne(ctx, "SELECT id, name, email, password, role FROM users WHERE id = ?", id)
}

// ListUsers returns every user ordered by ID.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, email, role FROM users ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (s *Store) scanOne(ctx context.Context, query string, arg any) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, query, arg).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user: %w", err)
	}
	return u, nil
}
