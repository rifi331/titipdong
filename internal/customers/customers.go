// Package customers persists the jastiper's customer directory.
package customers

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Customer is a person the jastiper buys on behalf of.
type Customer struct {
	ID          int64
	OwnerUserID int64
	Name        string
	WhatsApp    string
	Notes       string
	CreatedAt   time.Time
}

// Store wraps the database.
type Store struct{ pool *pgxpool.Pool }

// NewStore constructs a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// List returns customers owned by uid, ordered by name.
func (s *Store) List(ctx context.Context, ownerID int64) ([]Customer, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, owner_user_id, name, whatsapp, notes, created_at
		FROM customers WHERE owner_user_id = $1
		ORDER BY name`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Customer
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.OwnerUserID, &c.Name, &c.WhatsApp, &c.Notes, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get returns one customer, ensuring ownership.
func (s *Store) Get(ctx context.Context, ownerID, id int64) (Customer, error) {
	var c Customer
	err := s.pool.QueryRow(ctx, `
		SELECT id, owner_user_id, name, whatsapp, notes, created_at
		FROM customers WHERE id=$1 AND owner_user_id=$2`, id, ownerID).
		Scan(&c.ID, &c.OwnerUserID, &c.Name, &c.WhatsApp, &c.Notes, &c.CreatedAt)
	return c, err
}

// Create inserts a customer.
func (s *Store) Create(ctx context.Context, ownerID int64, name, whatsapp, notes string) (Customer, error) {
	var c Customer
	err := s.pool.QueryRow(ctx, `
		INSERT INTO customers (owner_user_id, name, whatsapp, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, owner_user_id, name, whatsapp, notes, created_at`,
		ownerID, name, whatsapp, notes).
		Scan(&c.ID, &c.OwnerUserID, &c.Name, &c.WhatsApp, &c.Notes, &c.CreatedAt)
	return c, err
}

// Update modifies a customer.
func (s *Store) Update(ctx context.Context, ownerID, id int64, name, whatsapp, notes string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE customers SET name=$1, whatsapp=$2, notes=$3
		WHERE id=$4 AND owner_user_id=$5`, name, whatsapp, notes, id, ownerID)
	return err
}

// Delete removes a customer.
func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM customers WHERE id=$1 AND owner_user_id=$2`, id, ownerID)
	return err
}
