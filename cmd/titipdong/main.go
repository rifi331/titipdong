// Command titipdong starts the jastip business tracker server.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"github.com/titipdong/titipdong/internal/config"
	"github.com/titipdong/titipdong/internal/db"
	"github.com/titipdong/titipdong/internal/web"
)

func main() {
	// Load .env if present. Existing env vars take precedence over file values,
	// so explicit exports / docker env still win. Missing file is fine.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	log.Println("database connected & migrated")

	if err := bootstrapAdmin(ctx, pool, cfg); err != nil {
		log.Printf("admin bootstrap skipped: %v", err)
	}

	srv := web.New(cfg, pool)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("titipdong listening on %s", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")

	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = srv.Shutdown(shutdownCtx)
	log.Println("bye")
}

// bootstrapAdmin creates an admin account from env on first boot, if configured.
func bootstrapAdmin(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return errors.New("ADMIN_EMAIL/ADMIN_PASSWORD not set")
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE role='admin'").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil // admin already exists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	name := cfg.AdminEmail
	if at := indexOfAt(name); at > 0 {
		name = name[:at]
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, role, display_name)
		VALUES ($1, $2, 'admin', $3)
		ON CONFLICT (email) DO UPDATE SET role='admin', password_hash=EXCLUDED.password_hash`,
		cfg.AdminEmail, hash, name)
	if err == nil {
		log.Printf("bootstrapped admin %s", cfg.AdminEmail)
	}
	return err
}

func indexOfAt(s string) int {
	for i, r := range s {
		if r == '@' {
			return i
		}
	}
	return -1
}
