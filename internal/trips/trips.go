// Package trips persists trips (one per travel/sourcing run).
package trips

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Status of a trip.
type Status string

const (
	StatusActive Status = "active"
	StatusClosed Status = "closed"
)

// Trip is a sourcing trip to a foreign country.
type Trip struct {
	ID          int64
	OwnerUserID int64
	Name        string
	Country     string
	Currency    string
	StartDate   *time.Time
	EndDate     *time.Time
	Status      Status
	CreatedAt   time.Time
}

// Store wraps the database.
type Store struct{ pool *pgxpool.Pool }

// NewStore constructs a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// List returns trips owned by uid.
func (s *Store) List(ctx context.Context, ownerID int64) ([]Trip, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, owner_user_id, name, country, currency, start_date, end_date, status, created_at
		FROM trips WHERE owner_user_id=$1 ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Trip
	for rows.Next() {
		var t Trip
		if err := rows.Scan(&t.ID, &t.OwnerUserID, &t.Name, &t.Country, &t.Currency,
			&t.StartDate, &t.EndDate, &t.Status, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns one trip.
func (s *Store) Get(ctx context.Context, ownerID, id int64) (Trip, error) {
	var t Trip
	err := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, name, country, currency, start_date, end_date, status, created_at
		FROM trips WHERE id=$1 AND owner_user_id=$2`, id, ownerID).
		Scan(&t.ID, &t.OwnerUserID, &t.Name, &t.Country, &t.Currency,
			&t.StartDate, &t.EndDate, &t.Status, &t.CreatedAt)
	return t, err
}

// Create inserts a trip.
func (s *Store) Create(ctx context.Context, t Trip) (Trip, error) {
	var out Trip
	err := s.pool.QueryRow(ctx, `
		INSERT INTO trips (owner_user_id, name, country, currency, start_date, end_date, status)
		VALUES ($1,$2,$3,$4,$5,$6, COALESCE(NULLIF($7,''),'active')::trip_status)
		RETURNING id, owner_user_id, name, country, currency, start_date, end_date, status, created_at`,
		t.OwnerUserID, t.Name, t.Country, t.Currency, t.StartDate, t.EndDate, string(t.Status)).
		Scan(&out.ID, &out.OwnerUserID, &out.Name, &out.Country, &out.Currency,
			&out.StartDate, &out.EndDate, &out.Status, &out.CreatedAt)
	return out, err
}

// Update modifies a trip.
func (s *Store) Update(ctx context.Context, t Trip) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE trips SET name=$1, country=$2, currency=$3, start_date=$4, end_date=$5, status=$6::trip_status
		WHERE id=$7 AND owner_user_id=$8`,
		t.Name, t.Country, t.Currency, t.StartDate, t.EndDate, string(t.Status), t.ID, t.OwnerUserID)
	return err
}

// SetStatus changes a trip's status.
func (s *Store) SetStatus(ctx context.Context, ownerID, id int64, st Status) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE trips SET status=$1::trip_status WHERE id=$2 AND owner_user_id=$3`, string(st), id, ownerID)
	return err
}
