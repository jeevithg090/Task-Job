package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"backend-auth/internal/auth"
	"backend-auth/internal/store"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

type testEnv struct {
	t      *testing.T
	srv    *httptest.Server
	store  *store.Store
	tokens *auth.TokenManager
}

func newTestEnv(t *testing.T, origins ...string) *testEnv {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	tokens := auth.NewTokenManager([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	s := NewServer(Options{
		Users:          st,
		Tokens:         tokens,
		AllowedOrigins: origins,
		Static:         fstest.MapFS{"index.html": {Data: []byte("<h1>tester</h1>")}},
	})
	hs := httptest.NewServer(s.Handler())
	t.Cleanup(func() {
		hs.Close()
		_ = st.Close()
	})
	return &testEnv{t: t, srv: hs, store: st, tokens: tokens}
}

func (e *testEnv) do(method, path, token string, body any) (*http.Response, map[string]any) {
	e.t.Helper()
	var buf bytes.Buffer
	switch b := body.(type) {
	case nil:
	case string:
		buf.WriteString(b)
	default:
		if err := json.NewEncoder(&buf).Encode(b); err != nil {
			e.t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, e.srv.URL+path, &buf)
	if err != nil {
		e.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res, out
}

func (e *testEnv) signup(name, email, password string) {
	e.t.Helper()
	res, body := e.do("POST", "/signup", "", map[string]string{"name": name, "email": email, "password": password})
	if res.StatusCode != http.StatusCreated {
		e.t.Fatalf("signup status = %d, body = %v", res.StatusCode, body)
	}
}

func (e *testEnv) login(email, password string) string {
	e.t.Helper()
	res, body := e.do("POST", "/login", "", map[string]string{"email": email, "password": password})
	if res.StatusCode != http.StatusOK {
		e.t.Fatalf("login status = %d, body = %v", res.StatusCode, body)
	}
	return body["token"].(string)
}

func TestSignupLoginProfileFlow(t *testing.T) {
	env := newTestEnv(t)

	res, body := env.do("POST", "/signup", "", map[string]string{
		"name": "  Ada  ", "email": " Ada@Example.COM ", "password": "password123",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("signup = %d %v", res.StatusCode, body)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q", ct)
	}
	user := body["user"].(map[string]any)
	if user["email"] != "ada@example.com" || user["name"] != "Ada" || user["role"] != "user" {
		t.Errorf("signup user = %v", user)
	}
	if _, leaked := user["password"]; leaked {
		t.Error("password must not be returned")
	}

	token := env.login("ADA@example.com", "password123")

	res, body = env.do("GET", "/profile", token, nil)
	if res.StatusCode != http.StatusOK || body["email"] != "ada@example.com" {
		t.Fatalf("profile = %d %v", res.StatusCode, body)
	}
}

func TestSignupValidation(t *testing.T) {
	env := newTestEnv(t)
	cases := map[string]struct {
		body   any
		status int
	}{
		"invalid json":       {`{"name":`, http.StatusBadRequest},
		"trailing data":      {`{"name":"a"} {}`, http.StatusBadRequest},
		"missing fields":     {map[string]string{"email": "a@example.com"}, http.StatusBadRequest},
		"bad email":          {map[string]string{"name": "A", "email": "nope", "password": "password123"}, http.StatusBadRequest},
		"short password":     {map[string]string{"name": "A", "email": "a@example.com", "password": "short"}, http.StatusBadRequest},
		"long password":      {map[string]string{"name": "A", "email": "a@example.com", "password": strings.Repeat("x", 73)}, http.StatusBadRequest},
		"admin self-signup":  {map[string]string{"name": "A", "email": "a@example.com", "password": "password123", "role": "admin"}, http.StatusForbidden},
		"unknown role":       {map[string]string{"name": "A", "email": "a@example.com", "password": "password123", "role": "root"}, http.StatusForbidden},
		"oversized body":     {`{"name":"` + strings.Repeat("a", maxBodyBytes) + `"}`, http.StatusBadRequest},
		"explicit user role": {map[string]string{"name": "A", "email": "ok@example.com", "password": "password123", "role": "user"}, http.StatusCreated},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res, body := env.do("POST", "/signup", "", tc.body)
			if res.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d (%v)", res.StatusCode, tc.status, body)
			}
			if tc.status >= 400 && body["error"] == "" {
				t.Error("error responses must carry an error message")
			}
		})
	}
}

func TestSignupDuplicateEmail(t *testing.T) {
	env := newTestEnv(t)
	env.signup("A", "dup@example.com", "password123")
	res, _ := env.do("POST", "/signup", "", map[string]string{"name": "B", "email": "DUP@example.com", "password": "password123"})
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", res.StatusCode)
	}
}

func TestLoginFailures(t *testing.T) {
	env := newTestEnv(t)
	env.signup("A", "a@example.com", "password123")

	for name, body := range map[string]map[string]string{
		"wrong password": {"email": "a@example.com", "password": "wrongpass"},
		"unknown email":  {"email": "ghost@example.com", "password": "password123"},
	} {
		t.Run(name, func(t *testing.T) {
			res, out := env.do("POST", "/login", "", body)
			if res.StatusCode != http.StatusUnauthorized || out["error"] != "invalid credentials" {
				t.Fatalf("got %d %v", res.StatusCode, out)
			}
		})
	}

	res, _ := env.do("POST", "/login", "", map[string]string{"email": "a@example.com"})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing password status = %d", res.StatusCode)
	}
}

func TestAuthMiddleware(t *testing.T) {
	env := newTestEnv(t)
	for name, header := range map[string]string{
		"missing":      "",
		"wrong scheme": "Basic abc",
		"empty token":  "Bearer ",
		"bad token":    "Bearer not.a.token",
	} {
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", env.srv.URL+"/profile", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = res.Body.Close()
			if res.StatusCode != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", res.StatusCode)
			}
		})
	}
}

func TestProfileForDeletedUser(t *testing.T) {
	env := newTestEnv(t)
	token, _ := env.tokens.Generate(999, auth.RoleUser)
	res, _ := env.do("GET", "/profile", token, nil)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", res.StatusCode)
	}
}

func TestListUsersRequiresAdmin(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	if err := EnsureAdmin(ctx, env.store, "Root", "root@example.com", "rootpassword"); err != nil {
		t.Fatalf("ensure admin: %v", err)
	}
	// Idempotent on restart.
	if err := EnsureAdmin(ctx, env.store, "Root", "root@example.com", "rootpassword"); err != nil {
		t.Fatalf("ensure admin again: %v", err)
	}
	env.signup("A", "a@example.com", "password123")

	userToken := env.login("a@example.com", "password123")
	res, _ := env.do("GET", "/users", userToken, nil)
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("user status = %d, want 403", res.StatusCode)
	}

	adminToken := env.login("root@example.com", "rootpassword")
	req, _ := http.NewRequest("GET", env.srv.URL+"/users", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	httpRes, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = httpRes.Body.Close() }()
	var users []map[string]any
	if err := json.NewDecoder(httpRes.Body).Decode(&users); err != nil {
		t.Fatal(err)
	}
	if httpRes.StatusCode != http.StatusOK || len(users) != 2 || users[0]["role"] != "admin" {
		t.Fatalf("admin list = %d %v", httpRes.StatusCode, users)
	}
}

func TestEnsureAdminRejectsInvalidSeed(t *testing.T) {
	env := newTestEnv(t)
	if err := EnsureAdmin(context.Background(), env.store, "Root", "root@example.com", "short"); err == nil {
		t.Fatal("expected weak admin password to be rejected")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	env := newTestEnv(t)
	res, _ := env.do("GET", "/signup", "", nil)
	if res.StatusCode != http.StatusMethodNotAllowed || res.Header.Get("Allow") != "POST" {
		t.Fatalf("status = %d, want 405", res.StatusCode)
	}
}

func TestHealthAndStatic(t *testing.T) {
	env := newTestEnv(t)
	res, body := env.do("GET", "/healthz", "", nil)
	if res.StatusCode != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("health = %d %v", res.StatusCode, body)
	}

	res, err := http.Get(env.srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("static = %d headers=%v", res.StatusCode, res.Header)
	}
}

func TestCORS(t *testing.T) {
	env := newTestEnv(t, "https://app.example")

	preflight := func(origin string) *http.Response {
		req, _ := http.NewRequest(http.MethodOptions, env.srv.URL+"/login", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res
	}

	if res := preflight("https://app.example"); res.StatusCode != http.StatusNoContent ||
		res.Header.Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("allowed preflight = %d %v", res.StatusCode, res.Header)
	}
	if res := preflight("https://evil.example"); res.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("disallowed origin got CORS headers: %v", res.Header)
	}
}
