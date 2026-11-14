package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestCreateAndFetchUser(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	created, err := s.CreateUser(ctx, "Ada", "ada@example.com", "hash", "user")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected an ID")
	}

	byEmail, err := s.UserByEmail(ctx, "ADA@Example.com")
	if err != nil {
		t.Fatalf("by email: %v", err)
	}
	if byEmail.ID != created.ID || byEmail.PasswordHash != "hash" {
		t.Errorf("by email = %+v", byEmail)
	}

	byID, err := s.UserByID(ctx, created.ID)
	if err != nil || byID.Email != "ada@example.com" {
		t.Errorf("by id = %+v, %v", byID, err)
	}
}

func TestDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if _, err := s.CreateUser(ctx, "A", "a@example.com", "h", "user"); err != nil {
		t.Fatal(err)
	}
	_, err := s.CreateUser(ctx, "B", "A@EXAMPLE.COM", "h", "user")
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
}

func TestNotFound(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.UserByID(context.Background(), 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("by id err = %v", err)
	}
	if _, err := s.UserByEmail(context.Background(), "x@example.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("by email err = %v", err)
	}
}

func TestListUsers(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	users, err := s.ListUsers(ctx)
	if err != nil || users == nil || len(users) != 0 {
		t.Fatalf("empty list = %#v, %v (want non-nil empty slice)", users, err)
	}

	for _, e := range []string{"a@example.com", "b@example.com"} {
		if _, err := s.CreateUser(ctx, "n", e, "h", "user"); err != nil {
			t.Fatal(err)
		}
	}
	users, err = s.ListUsers(ctx)
	if err != nil || len(users) != 2 || users[0].Email != "a@example.com" {
		t.Fatalf("list = %+v, %v", users, err)
	}
	if users[0].PasswordHash != "" {
		t.Error("list must not load password hashes")
	}
}
