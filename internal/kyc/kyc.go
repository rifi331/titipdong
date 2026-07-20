// Package kyc persists "Become a Jastiper" applications and admin decisions.
package kyc

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Status of an application.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// Application is the KYC form a buyer submits to become a jastiper.
type Application struct {
	ID               int64
	UserID           int64
	FullName         string
	KTPNumber        string
	KTPPhotoPath     string
	Phone            string
	City             string
	Status           Status
	ReviewedByUserID *int64
	ReviewedAt       *time.Time
	AdminNote        string
	CreatedAt        time.Time

	// Joined
	UserEmail string
}

// Store wraps the database.
type Store struct{ pool *pgxpool.Pool }

// NewStore constructs a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Submit creates a pending application for a user.
func (s *Store) Submit(ctx context.Context, a Application) (Application, error) {
	var out Application
	err := s.pool.QueryRow(ctx, `
		INSERT INTO jastiper_applications
		  (user_id, full_name, ktp_number, ktp_photo_path, phone, city, status)
		VALUES ($1,$2,$3,$4,$5,$6,'pending')
		RETURNING id, user_id, full_name, ktp_number, ktp_photo_path, phone, city, status::text,
		          reviewed_by_user_id, reviewed_at, admin_note, created_at`,
		a.UserID, a.FullName, a.KTPNumber, a.KTPPhotoPath, a.Phone, a.City).
		Scan(&out.ID, &out.UserID, &out.FullName, &out.KTPNumber, &out.KTPPhotoPath, &out.Phone, &out.City,
			&out.Status, &out.ReviewedByUserID, &out.ReviewedAt, &out.AdminNote, &out.CreatedAt)
	return out, err
}

// LatestForUser returns the user's most recent application, if any.
func (s *Store) LatestForUser(ctx context.Context, userID int64) (Application, bool, error) {
	var a Application
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, full_name, ktp_number, ktp_photo_path, phone, city, status::text,
		       reviewed_by_user_id, reviewed_at, admin_note, created_at
		FROM jastiper_applications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1`, userID).
		Scan(&a.ID, &a.UserID, &a.FullName, &a.KTPNumber, &a.KTPPhotoPath, &a.Phone, &a.City,
			&a.Status, &a.ReviewedByUserID, &a.ReviewedAt, &a.AdminNote, &a.CreatedAt)
	if err != nil {
		return Application{}, false, nil // missing -> no application yet
	}
	return a, true, nil
}

// ListPending returns applications awaiting review, newest first.
func (s *Store) ListPending(ctx context.Context) ([]Application, error) {
	return s.listByStatus(ctx, "pending")
}

// ListAll returns every application.
func (s *Store) ListAll(ctx context.Context) ([]Application, error) {
	return s.listByStatus(ctx, "")
}

func (s *Store) listByStatus(ctx context.Context, status string) ([]Application, error) {
	q := `
		SELECT a.id, a.user_id, a.full_name, a.ktp_number, a.ktp_photo_path, a.phone, a.city,
		       a.status::text, a.reviewed_by_user_id, a.reviewed_at, a.admin_note, a.created_at,
		       COALESCE(u.email,'')
		FROM jastiper_applications a
		JOIN users u ON u.id = a.user_id`
	args := []any{}
	if status != "" {
		args = append(args, status)
		q += " WHERE a.status = $1::application_status"
	}
	q += " ORDER BY a.created_at DESC"
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Application
	for rows.Next() {
		var a Application
		if err := rows.Scan(&a.ID, &a.UserID, &a.FullName, &a.KTPNumber, &a.KTPPhotoPath, &a.Phone, &a.City,
			&a.Status, &a.ReviewedByUserID, &a.ReviewedAt, &a.AdminNote, &a.CreatedAt, &a.UserEmail); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Get returns a single application by id.
func (s *Store) Get(ctx context.Context, id int64) (Application, error) {
	var a Application
	err := s.pool.QueryRow(ctx, `
		SELECT a.id, a.user_id, a.full_name, a.ktp_number, a.ktp_photo_path, a.phone, a.city,
		       a.status::text, a.reviewed_by_user_id, a.reviewed_at, a.admin_note, a.created_at,
		       COALESCE(u.email,'')
		FROM jastiper_applications a
		JOIN users u ON u.id = a.user_id
		WHERE a.id = $1`, id).
		Scan(&a.ID, &a.UserID, &a.FullName, &a.KTPNumber, &a.KTPPhotoPath, &a.Phone, &a.City,
			&a.Status, &a.ReviewedByUserID, &a.ReviewedAt, &a.AdminNote, &a.CreatedAt, &a.UserEmail)
	return a, err
}

// Decide sets the application status and (on approval) promotes the user to jastiper.
// Performed in a single transaction so role only flips when the row updates.
func (s *Store) Decide(ctx context.Context, appID, reviewerID int64, approve bool, note string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	status := StatusRejected
	if approve {
		status = StatusApproved
	}

	var userID int64
	err = tx.QueryRow(ctx, `
		UPDATE jastiper_applications
		SET status = $1::application_status, reviewed_by_user_id = $2, reviewed_at = now(), admin_note = $3
		WHERE id = $4 AND status = 'pending'
		RETURNING user_id`, string(status), reviewerID, note, appID).Scan(&userID)
	if err != nil {
		return err
	}
	if approve {
		if _, err := tx.Exec(ctx, `UPDATE users SET role = 'jastiper' WHERE id = $1`, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
