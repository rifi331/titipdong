// Package requests persists anonymous buyer requests from the public catalog.
//
// A buyer (no account needed) taps "Mau Ini!" on a catalog item and submits
// their name + WhatsApp + optional note. The request lands in the owning
// jastiper's queue. On accept, the jastiper converts it into a real order +
// customer row (see web handlers for the conversion transaction).
package requests

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Status of a buyer request.
type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusRejected Status = "rejected"
)

// ErrAlreadyProcessed is returned by SetStatus when the request is no longer
// pending (already accepted or rejected). Lets callers handle it idempotently.
var ErrAlreadyProcessed = errors.New("request already processed")

// ErrNotFound is returned when a request doesn't exist (or isn't owned by the
// caller, so we never leak existence across owners).
var ErrNotFound = errors.New("request not found")

// Request is an anonymous buyer's interest in a catalog item.
type Request struct {
	ID                int64
	CatalogItemID     int64
	JastiperUserID    int64
	BuyerName         string
	BuyerWhatsApp     string
	BuyerNote         string
	Status            Status
	ConvertedOrderID  *int64
	ConvertedCustomer *int64
	CreatedAt         time.Time

	// Joined fields for display (filled by queries that JOIN catalog/users/kyc).
	ItemTitle         string
	ItemDescription   string
	ItemCurrency      string
	ItemEstPriceForeign float64
	ItemPhotoPath     string
	JastiperName      string
	JastiperPhone     string
}

// Store wraps the database.
type Store struct{ pool *pgxpool.Pool }

// NewStore constructs a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Submit inserts a new pending request and returns it with id + created_at.
func (s *Store) Submit(ctx context.Context, r Request) (Request, error) {
	var out Request
	err := s.pool.QueryRow(ctx, `
		INSERT INTO buyer_requests (catalog_item_id, jastiper_user_id, buyer_name, buyer_whatsapp, buyer_note, status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
		RETURNING id, catalog_item_id, jastiper_user_id, buyer_name, buyer_whatsapp, buyer_note,
		          status::text, converted_order_id, converted_customer_id, created_at`,
		r.CatalogItemID, r.JastiperUserID, r.BuyerName, r.BuyerWhatsApp, r.BuyerNote).
		Scan(&out.ID, &out.CatalogItemID, &out.JastiperUserID, &out.BuyerName, &out.BuyerWhatsApp, &out.BuyerNote,
			&out.Status, &out.ConvertedOrderID, &out.ConvertedCustomer, &out.CreatedAt)
	return out, err
}

// GetWithItem loads a request joined with its catalog item + jastiper info.
// Used by the public submit handler to build the WA link and confirmation page.
// Not owner-scoped (callers pass the catalog item id, which already encodes ownership).
func (s *Store) GetWithItem(ctx context.Context, id int64) (Request, error) {
	var r Request
	err := s.pool.QueryRow(ctx, `
		SELECT r.id, r.catalog_item_id, r.jastiper_user_id, r.buyer_name, r.buyer_whatsapp,
		       r.buyer_note, r.status::text, r.converted_order_id, r.converted_customer_id, r.created_at,
		       c.title, c.description, c.currency, c.est_price_foreign, c.photo_path,
		       COALESCE(u.display_name, u.email),
		       COALESCE((SELECT phone FROM jastiper_applications a
		                 WHERE a.user_id = r.jastiper_user_id AND a.status='approved'
		                 ORDER BY created_at DESC LIMIT 1), '')
		FROM buyer_requests r
		JOIN catalog_items c ON c.id = r.catalog_item_id
		JOIN users u         ON u.id = r.jastiper_user_id
		WHERE r.id = $1`, id).
		Scan(&r.ID, &r.CatalogItemID, &r.JastiperUserID, &r.BuyerName, &r.BuyerWhatsApp,
			&r.BuyerNote, &r.Status, &r.ConvertedOrderID, &r.ConvertedCustomer, &r.CreatedAt,
			&r.ItemTitle, &r.ItemDescription, &r.ItemCurrency, &r.ItemEstPriceForeign, &r.ItemPhotoPath,
			&r.JastiperName, &r.JastiperPhone)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	return r, err
}

// ListForJastiper returns all requests owned by a jastiper (newest first),
// joined with catalog item details for display.
func (s *Store) ListForJastiper(ctx context.Context, jastiperID int64) ([]Request, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.catalog_item_id, r.jastiper_user_id, r.buyer_name, r.buyer_whatsapp,
		       r.buyer_note, r.status::text, r.converted_order_id, r.converted_customer_id, r.created_at,
		       c.title, c.description, c.currency, c.est_price_foreign, c.photo_path,
		       '', ''
		FROM buyer_requests r
		JOIN catalog_items c ON c.id = r.catalog_item_id
		WHERE r.jastiper_user_id = $1
		ORDER BY r.created_at DESC`, jastiperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows)
}

// ListPending returns only pending requests for a jastiper (for the dashboard badge + queue).
func (s *Store) ListPending(ctx context.Context, jastiperID int64) ([]Request, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.catalog_item_id, r.jastiper_user_id, r.buyer_name, r.buyer_whatsapp,
		       r.buyer_note, r.status::text, r.converted_order_id, r.converted_customer_id, r.created_at,
		       c.title, c.description, c.currency, c.est_price_foreign, c.photo_path,
		       '', ''
		FROM buyer_requests r
		JOIN catalog_items c ON c.id = r.catalog_item_id
		WHERE r.jastiper_user_id = $1 AND r.status = 'pending'
		ORDER BY r.created_at DESC`, jastiperID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows)
}

// CountPending returns the number of pending requests for a jastiper (for nav badge).
func (s *Store) CountPending(ctx context.Context, jastiperID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM buyer_requests WHERE jastiper_user_id=$1 AND status='pending'`,
		jastiperID).Scan(&n)
	return n, err
}

// Get returns one request, owner-scoped. Joined with item details.
func (s *Store) Get(ctx context.Context, jastiperID, id int64) (Request, error) {
	var r Request
	err := s.pool.QueryRow(ctx, `
		SELECT r.id, r.catalog_item_id, r.jastiper_user_id, r.buyer_name, r.buyer_whatsapp,
		       r.buyer_note, r.status::text, r.converted_order_id, r.converted_customer_id, r.created_at,
		       c.title, c.description, c.currency, c.est_price_foreign, c.photo_path,
		       COALESCE(u.display_name, u.email),
		       COALESCE((SELECT phone FROM jastiper_applications a
		                 WHERE a.user_id = r.jastiper_user_id AND a.status='approved'
		                 ORDER BY created_at DESC LIMIT 1), '')
		FROM buyer_requests r
		JOIN catalog_items c ON c.id = r.catalog_item_id
		JOIN users u         ON u.id = r.jastiper_user_id
		WHERE r.id = $1 AND r.jastiper_user_id = $2`, id, jastiperID).
		Scan(&r.ID, &r.CatalogItemID, &r.JastiperUserID, &r.BuyerName, &r.BuyerWhatsApp,
			&r.BuyerNote, &r.Status, &r.ConvertedOrderID, &r.ConvertedCustomer, &r.CreatedAt,
			&r.ItemTitle, &r.ItemDescription, &r.ItemCurrency, &r.ItemEstPriceForeign, &r.ItemPhotoPath,
			&r.JastiperName, &r.JastiperPhone)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	return r, err
}

// SetStatus transitions a pending request to accepted or rejected.
// Idempotent: returns ErrAlreadyProcessed if the request is no longer pending.
func (s *Store) SetStatus(ctx context.Context, jastiperID, id int64, status Status) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE buyer_requests SET status = $1::request_status
		WHERE id = $2 AND jastiper_user_id = $3 AND status = 'pending'`,
		string(status), id, jastiperID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// Either the request doesn't exist or it's already processed.
		// Either way, treat as already-processed so callers can handle idempotently.
		return ErrAlreadyProcessed
	}
	return nil
}

// MarkConverted records the order + customer rows created on acceptance.
// Called after the accept transaction commits, so failure here is non-fatal
// (the conversion already happened; we just lose the back-reference).
func (s *Store) MarkConverted(ctx context.Context, id, orderID, customerID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE buyer_requests SET converted_order_id=$1, converted_customer_id=$2 WHERE id=$3`,
		orderID, customerID, id)
	return err
}

func collect(rows pgx.Rows) ([]Request, error) {
	var out []Request
	for rows.Next() {
		var r Request
		if err := rows.Scan(&r.ID, &r.CatalogItemID, &r.JastiperUserID, &r.BuyerName, &r.BuyerWhatsApp,
			&r.BuyerNote, &r.Status, &r.ConvertedOrderID, &r.ConvertedCustomer, &r.CreatedAt,
			&r.ItemTitle, &r.ItemDescription, &r.ItemCurrency, &r.ItemEstPriceForeign, &r.ItemPhotoPath,
			&r.JastiperName, &r.JastiperPhone); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
