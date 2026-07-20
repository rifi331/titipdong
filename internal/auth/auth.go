// Package auth handles users, sessions, passwords, and HTTP middleware.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Role is a user's capability level.
type Role string

const (
	RoleBuyer    Role = "buyer"
	RoleJastiper Role = "jastiper"
	RoleAdmin    Role = "admin"
)

// User is an authenticated account.
type User struct {
	ID           int64
	Email        string
	PasswordHash []byte
	Role         Role
	DisplayName  string
}

// ErrInvalidCredentials is returned on bad login.
var ErrInvalidCredentials = errors.New("invalid credentials")

// Service wraps user persistence and session management.
type Service struct {
	pool     *pgxpool.Pool
	sessions *scs.SessionManager
}

// New constructs an auth Service.
func New(pool *pgxpool.Pool, sessions *scs.SessionManager) *Service {
	return &Service{pool: pool, sessions: sessions}
}

// Create inserts a new buyer-scoped user and returns its id.
func (s *Service) Create(ctx context.Context, email, password, displayName string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return User{}, errors.New("email dan password wajib diisi")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	const q = `
		INSERT INTO users (email, password_hash, role, display_name)
		VALUES ($1, $2, 'buyer', $3)
		RETURNING id, email, password_hash, role, display_name`
	var u User
	err = s.pool.QueryRow(ctx, q, email, hash, strings.TrimSpace(displayName)).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.DisplayName)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

// FindByEmail loads a user by email (case-insensitive).
func (s *Service) FindByEmail(ctx context.Context, email string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, display_name
		FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.DisplayName)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// FindByID loads a user by id.
func (s *Service) FindByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, display_name
		FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.DisplayName)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// CountAdmins returns the number of admin accounts.
func (s *Service) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&n)
	return n, err
}

// SetRole promotes/demotes a user. Only admin should call this.
func (s *Service) SetRole(ctx context.Context, userID int64, role Role) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET role = $1 WHERE id = $2`, role, userID)
	return err
}

// VerifyPassword checks the password against the stored hash.
func VerifyPassword(hash, password []byte) error {
	return bcrypt.CompareHashAndPassword(hash, password)
}

// Login authenticates and stores the user id in the session.
func (s *Service) Login(ctx context.Context, w http.ResponseWriter, r *http.Request, email, password string) error {
	u, err := s.FindByEmail(ctx, email)
	if err != nil {
		return err
	}
	if err := VerifyPassword(u.PasswordHash, []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	s.sessions.Put(ctx, "userID", u.ID)
	s.sessions.Put(ctx, "role", string(u.Role))
	return nil
}

// Logout clears the session.
func (s *Service) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return s.sessions.Destroy(ctx)
}

// FromContext returns the authenticated user for the request, if any.
func (s *Service) FromContext(ctx context.Context) (User, bool) {
	id, ok := s.sessions.Get(ctx, "userID").(int64)
	if !ok || id == 0 {
		return User{}, false
	}
	u, err := s.FindByID(ctx, id)
	if err != nil {
		return User{}, false
	}
	return u, true
}

// LoadUser is middleware that attaches *User to request context under userKey.
func (s *Service) LoadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.FromContext(r.Context())
		if ok {
			ctx := context.WithValue(r.Context(), userKey{}, &u)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth rejects unauthenticated requests.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFrom(r); !ok {
			http.Redirect(w, r, "/login?next="+r.URL.Path, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole allows the listed roles (and always admin).
func (s *Service) RequireRole(roles ...Role) func(http.Handler) http.Handler {
	allowed := map[Role]bool{RoleAdmin: true}
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFrom(r)
			if !ok || !allowed[u.Role] {
				http.Error(w, "Akses ditolak", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type userKey struct{}

// UserFrom extracts the authenticated user from the request context.
func UserFrom(r *http.Request) (User, bool) {
	u, ok := r.Context().Value(userKey{}).(*User)
	if !ok || u == nil {
		return User{}, false
	}
	return *u, true
}
