// Package trips persists trips (one per travel/sourcing run).
//
// A trip is the jastiper's sourcing mission: destination, dates, weight/slot
// limits, order cutoff time, and a 3-stage lifecycle:
//
//	on_plan -> at_destination_country -> in_home_country
package trips

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Status of a trip.
type Status string

const (
	StatusOnPlan                Status = "on_plan"
	StatusAtDestinationCountry  Status = "at_destination_country"
	StatusInHomeCountry         Status = "in_home_country"
)

// Trip is a sourcing trip to a foreign country.
type Trip struct {
	ID                      int64
	OwnerUserID             int64
	Title                   string
	DestinationCountry      string
	DestinationCity         string
	Currency                string
	DepartureDate           *time.Time
	ReturnDate              *time.Time
	OrderCutoffAt           *time.Time
	EstimatedDelivery       *time.Time
	MaxWeightKg             float64
	UsedWeightKg            float64
	MaxItemSlots            int
	Notes                   string
	Status                  Status
	CreatedAt               time.Time
}

// Store wraps the database.
type Store struct{ pool *pgxpool.Pool }

// NewStore constructs a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const tripColumns = `
	id, owner_user_id, title, destination_country, destination_city, currency,
	departure_date, return_date, order_cutoff_at, estimated_delivery,
	max_weight_kg, used_weight_kg, max_item_slots, notes,
	status::text, created_at`

func scanTrip(row interface{ Scan(...any) error }) (Trip, error) {
	var t Trip
	err := row.Scan(&t.ID, &t.OwnerUserID, &t.Title, &t.DestinationCountry, &t.DestinationCity,
		&t.Currency, &t.DepartureDate, &t.ReturnDate, &t.OrderCutoffAt, &t.EstimatedDelivery,
		&t.MaxWeightKg, &t.UsedWeightKg, &t.MaxItemSlots, &t.Notes,
		&t.Status, &t.CreatedAt)
	return t, err
}

// List returns trips owned by uid.
func (s *Store) List(ctx context.Context, ownerID int64) ([]Trip, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+tripColumns+` FROM trips WHERE owner_user_id=$1 ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Trip
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListAll returns trips for all jastipers (admin view), optionally filtered.
func (s *Store) ListAll(ctx context.Context, ownerID *int64) ([]Trip, error) {
	q := `SELECT ` + tripColumns + ` FROM trips WHERE 1=1`
	args := []any{}
	if ownerID != nil {
		args = append(args, *ownerID)
		q += ` AND owner_user_id = $1`
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Trip
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get returns one trip.
func (s *Store) Get(ctx context.Context, ownerID, id int64) (Trip, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+tripColumns+` FROM trips WHERE id=$1 AND owner_user_id=$2`, id, ownerID)
	return scanTrip(row)
}

// GetByID returns one trip by id (admin scope).
func (s *Store) GetByID(ctx context.Context, id int64) (Trip, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+tripColumns+` FROM trips WHERE id=$1`, id)
	return scanTrip(row)
}

// Create inserts a trip.
func (s *Store) Create(ctx context.Context, t Trip) (Trip, error) {
	if t.Status == "" {
		t.Status = StatusOnPlan
	}
	var out Trip
	err := s.pool.QueryRow(ctx, `
		INSERT INTO trips
		  (owner_user_id, title, destination_country, destination_city, currency,
		   departure_date, return_date, order_cutoff_at, estimated_delivery,
		   max_weight_kg, used_weight_kg, max_item_slots, notes, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::trip_status)
		RETURNING `+tripColumns,
		t.OwnerUserID, t.Title, t.DestinationCountry, t.DestinationCity, t.Currency,
		t.DepartureDate, t.ReturnDate, t.OrderCutoffAt, t.EstimatedDelivery,
		t.MaxWeightKg, t.UsedWeightKg, t.MaxItemSlots, t.Notes, string(t.Status)).
		Scan(&out.ID, &out.OwnerUserID, &out.Title, &out.DestinationCountry, &out.DestinationCity,
			&out.Currency, &out.DepartureDate, &out.ReturnDate, &out.OrderCutoffAt, &out.EstimatedDelivery,
			&out.MaxWeightKg, &out.UsedWeightKg, &out.MaxItemSlots, &out.Notes,
			&out.Status, &out.CreatedAt)
	return out, err
}

// Update modifies a trip.
func (s *Store) Update(ctx context.Context, t Trip) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE trips SET
		  title=$1, destination_country=$2, destination_city=$3, currency=$4,
		  departure_date=$5, return_date=$6, order_cutoff_at=$7, estimated_delivery=$8,
		  max_weight_kg=$9, max_item_slots=$10, notes=$11,
		  status=$12::trip_status
		WHERE id=$13 AND owner_user_id=$14`,
		t.Title, t.DestinationCountry, t.DestinationCity, t.Currency,
		t.DepartureDate, t.ReturnDate, t.OrderCutoffAt, t.EstimatedDelivery,
		t.MaxWeightKg, t.MaxItemSlots, t.Notes,
		string(t.Status), t.ID, t.OwnerUserID)
	return err
}

// SetStatus changes a trip's status.
func (s *Store) SetStatus(ctx context.Context, ownerID, id int64, st Status) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE trips SET status=$1::trip_status WHERE id=$2 AND owner_user_id=$3`,
		string(st), id, ownerID)
	return err
}

// Delete removes a trip.
func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM trips WHERE id=$1 AND owner_user_id=$2`, id, ownerID)
	return err
}

// StatusLabel returns a short human label for display.
func StatusLabel(st Status) string {
	switch st {
	case StatusOnPlan:
		return "On Plan"
	case StatusAtDestinationCountry:
		return "At Destination"
	case StatusInHomeCountry:
		return "In Home Country"
	}
	return string(st)
}

// StatusEmoji gives a glyph for quick scanning.
func StatusEmoji(st Status) string {
	switch st {
	case StatusOnPlan:
		return "📋"
	case StatusAtDestinationCountry:
		return "✈️"
	case StatusInHomeCountry:
		return "🏠"
	}
	return ""
}
