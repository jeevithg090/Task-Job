// Command backend-auth runs the authentication API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend-auth/internal/api"
	"backend-auth/internal/auth"
	"backend-auth/internal/config"
	"backend-auth/internal/store"
	"backend-auth/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	users, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer func() { _ = users.Close() }()

	if cfg.Admin.Enabled() {
		if err := api.EnsureAdmin(ctx, users, cfg.Admin.Name, cfg.Admin.Email, cfg.Admin.Password); err != nil {
			return err
		}
	}

	srv := api.NewServer(api.Options{
		Users:          users,
		Tokens:         auth.NewTokenManager(cfg.JWTSecret, cfg.TokenTTL),
		AllowedOrigins: cfg.AllowedOrigins,
		Static:         web.Static(),
	})

	httpServer := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", "http://localhost:"+cfg.Port)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
