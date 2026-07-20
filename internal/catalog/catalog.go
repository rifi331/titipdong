// Package catalog stores browseable items offered by jastipers.
package catalog

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Status of a catalog item.
type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

// Item is a jastiper's offered product, visible to buyers.
type Item struct {
	ID              int64
	JastiperUserID  int64
	Title           string
	Description     string
	EstPriceForeign float64
	Currency        string
	PhotoPath       string
	Status          Status
	CreatedAt       time.Time

	// Joined
	JastiperName string
}

// Store wraps the database.
type Store struct{ pool *pgxpool.Pool }

// NewStore constructs a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// ListPublic returns active items for buyers to browse.
func (s *Store) ListPublic(ctx context.Context) ([]Item, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, c.jastiper_user_id, c.title, c.description, c.est_price_foreign,
		       c.currency, c.photo_path, c.status::text, c.created_at,
		       COALESCE(u.display_name, u.email)
		FROM catalog_items c
		JOIN users u ON u.id = c.jastiper_user_id
		WHERE c.status = 'active'
		ORDER BY c.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.JastiperUserID, &it.Title, &it.Description, &it.EstPriceForeign,
			&it.Currency, &it.PhotoPath, &it.Status, &it.CreatedAt, &it.JastiperName); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ListByOwner returns items belonging to a jastiper.
func (s *Store) ListByOwner(ctx context.Context, ownerID int64) ([]Item, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, jastiper_user_id, title, description, est_price_foreign,
		       currency, photo_path, status::text, created_at
		FROM catalog_items WHERE jastiper_user_id=$1
		ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.JastiperUserID, &it.Title, &it.Description, &it.EstPriceForeign,
			&it.Currency, &it.PhotoPath, &it.Status, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Create inserts a catalog item.
func (s *Store) Create(ctx context.Context, it Item) (Item, error) {
	if it.Status == "" {
		it.Status = StatusActive
	}
	var out Item
	err := s.pool.QueryRow(ctx, `
		INSERT INTO catalog_items (jastiper_user_id, title, description, est_price_foreign, currency, photo_path, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7::catalog_status)
		RETURNING id, jastiper_user_id, title, description, est_price_foreign, currency, photo_path, status::text, created_at`,
		it.JastiperUserID, it.Title, it.Description, it.EstPriceForeign, it.Currency, it.PhotoPath, string(it.Status)).
		Scan(&out.ID, &out.JastiperUserID, &out.Title, &out.Description, &out.EstPriceForeign,
			&out.Currency, &out.PhotoPath, &out.Status, &out.CreatedAt)
	return out, err
}

// Delete removes a catalog item.
func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM catalog_items WHERE id=$1 AND jastiper_user_id=$2`, id, ownerID)
	return err
}

// GetPublic loads a single active catalog item by id (for the public request form).
// Returns ErrNotFound if the item doesn't exist or is archived.
func (s *Store) GetPublic(ctx context.Context, id int64) (Item, error) {
	var it Item
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.jastiper_user_id, c.title, c.description, c.est_price_foreign,
		       c.currency, c.photo_path, c.status::text, c.created_at,
		       COALESCE(u.display_name, u.email)
		FROM catalog_items c
		JOIN users u ON u.id = c.jastiper_user_id
		WHERE c.id = $1 AND c.status = 'active'`, id).
		Scan(&it.ID, &it.JastiperUserID, &it.Title, &it.Description, &it.EstPriceForeign,
			&it.Currency, &it.PhotoPath, &it.Status, &it.CreatedAt, &it.JastiperName)
	if err != nil {
		return Item{}, ErrNotFound
	}
	return it, nil
}

// ErrNotFound is returned by GetPublic when no active item matches the id.
var ErrNotFound = errors.New("catalog item not found")
