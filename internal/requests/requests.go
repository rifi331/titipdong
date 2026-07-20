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

// FeeModel is how the jastiper charges for a custom request.
type FeeModel string

const (
	FeePercent FeeModel = "percent" // fee = est_price × fx × fee_percent/100
	FeePerKg   FeeModel = "per_kg"  // fee = est_weight_kg × fee_per_kg_idr
)

// Request is an anonymous buyer's interest in an item. Two flavors:
//   - Catalog: CatalogItemID set, item_* copied from the catalog row.
//   - Custom:  CatalogItemID nil, buyer supplies item_* themselves.
type Request struct {
	ID                int64
	CatalogItemID     *int64 // nil for custom requests
	JastiperUserID    int64
	BuyerName         string
	BuyerWhatsApp     string
	BuyerNote         string
	Status            Status
	ConvertedOrderID  *int64
	ConvertedCustomer *int64
	CreatedAt         time.Time

	// Item snapshot (always populated, regardless of catalog vs custom).
	ItemTitle        string
	ItemDescription  string
	ItemCurrency     string
	ItemEstPrice     float64
	ItemOrigin       string  // country/city of origin
	ItemEstWeightKg  float64

	// Fee model — set by jastiper on accept. Nil until then.
	FeeModel     *FeeModel
	FeePercent   *float64
	FeePerKgIDR  *float64

	// Display-only joined fields.
	ItemPhotoPath string
	JastiperName  string
	JastiperPhone string
}

// IsCustom reports whether this is a buyer-described item (no catalog reference).
func (r Request) IsCustom() bool { return r.CatalogItemID == nil }

// Store wraps the database.
type Store struct{ pool *pgxpool.Pool }

// NewStore constructs a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Submit inserts a new pending request and returns it with id + created_at.
// For catalog requests, CatalogItemID is set and item_* snapshot is taken from
// the catalog row. For custom requests, CatalogItemID is nil and item_* come
// from the buyer's form input.
func (s *Store) Submit(ctx context.Context, r Request) (Request, error) {
	var out Request
	err := s.pool.QueryRow(ctx, `
		INSERT INTO buyer_requests
		  (catalog_item_id, jastiper_user_id, buyer_name, buyer_whatsapp, buyer_note,
		   item_title, item_description, item_currency, item_est_price, item_origin, item_est_weight_kg,
		   status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'pending')
		RETURNING id, catalog_item_id, jastiper_user_id, buyer_name, buyer_whatsapp, buyer_note,
		          item_title, item_description, item_currency, item_est_price, item_origin, item_est_weight_kg,
		          status::text, converted_order_id, converted_customer_id, created_at`,
		r.CatalogItemID, r.JastiperUserID, r.BuyerName, r.BuyerWhatsApp, r.BuyerNote,
		r.ItemTitle, r.ItemDescription, r.ItemCurrency, r.ItemEstPrice, r.ItemOrigin, r.ItemEstWeightKg).
		Scan(&out.ID, &out.CatalogItemID, &out.JastiperUserID, &out.BuyerName, &out.BuyerWhatsApp, &out.BuyerNote,
			&out.ItemTitle, &out.ItemDescription, &out.ItemCurrency, &out.ItemEstPrice, &out.ItemOrigin, &out.ItemEstWeightKg,
			&out.Status, &out.ConvertedOrderID, &out.ConvertedCustomer, &out.CreatedAt)
	return out, err
}

// selectColumns is the canonical column list for all read queries.
// Uses LEFT JOIN on catalog_items so custom requests (catalog_item_id NULL)
// still resolve their item_* snapshot from the request row itself.
const selectColumns = `
	r.id, r.catalog_item_id, r.jastiper_user_id, r.buyer_name, r.buyer_whatsapp, r.buyer_note,
	COALESCE(r.item_title,       c.title),            COALESCE(r.item_description, c.description),
	COALESCE(r.item_currency,    c.currency),         COALESCE(r.item_est_price,  c.est_price_foreign),
	r.item_origin, r.item_est_weight_kg,
	COALESCE(c.photo_path, ''), r.status::text, r.converted_order_id, r.converted_customer_id, r.created_at,
	r.fee_model::text, r.fee_percent, r.fee_per_kg_idr,
	COALESCE(u.display_name, u.email),
	COALESCE((SELECT phone FROM jastiper_applications a
	          WHERE a.user_id = r.jastiper_user_id AND a.status='approved'
	          ORDER BY created_at DESC LIMIT 1), '')`

// GetWithItem loads a request joined with its catalog item (if any) + jastiper info.
// Used by the public submit handler to build the WA link and confirmation page.
// Not owner-scoped (callers pass the request id, used post-submit).
func (s *Store) GetWithItem(ctx context.Context, id int64) (Request, error) {
	var r Request
	err := s.pool.QueryRow(ctx, `
		SELECT `+selectColumns+`
		FROM buyer_requests r
		LEFT JOIN catalog_items c ON c.id = r.catalog_item_id
		LEFT JOIN users u         ON u.id = r.jastiper_user_id
		WHERE r.id = $1`, id).
		Scan(&r.ID, &r.CatalogItemID, &r.JastiperUserID, &r.BuyerName, &r.BuyerWhatsApp, &r.BuyerNote,
			&r.ItemTitle, &r.ItemDescription, &r.ItemCurrency, &r.ItemEstPrice,
			&r.ItemOrigin, &r.ItemEstWeightKg,
			&r.ItemPhotoPath, &r.Status, &r.ConvertedOrderID, &r.ConvertedCustomer, &r.CreatedAt,
			&r.FeeModel, &r.FeePercent, &r.FeePerKgIDR,
			&r.JastiperName, &r.JastiperPhone)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	return r, err
}

// ListForJastiper returns all requests owned by a jastiper (newest first).
func (s *Store) ListForJastiper(ctx context.Context, jastiperID int64) ([]Request, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+selectColumns+`
		FROM buyer_requests r
		LEFT JOIN catalog_items c ON c.id = r.catalog_item_id
		LEFT JOIN users u         ON u.id = r.jastiper_user_id
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
		SELECT `+selectColumns+`
		FROM buyer_requests r
		LEFT JOIN catalog_items c ON c.id = r.catalog_item_id
		LEFT JOIN users u         ON u.id = r.jastiper_user_id
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

// Get returns one request, owner-scoped. Joined with item details + jastiper info.
func (s *Store) Get(ctx context.Context, jastiperID, id int64) (Request, error) {
	var r Request
	err := s.pool.QueryRow(ctx, `
		SELECT `+selectColumns+`
		FROM buyer_requests r
		LEFT JOIN catalog_items c ON c.id = r.catalog_item_id
		LEFT JOIN users u         ON u.id = r.jastiper_user_id
		WHERE r.id = $1 AND r.jastiper_user_id = $2`, id, jastiperID).
		Scan(&r.ID, &r.CatalogItemID, &r.JastiperUserID, &r.BuyerName, &r.BuyerWhatsApp,
			&r.BuyerNote, &r.ItemTitle, &r.ItemDescription, &r.ItemCurrency, &r.ItemEstPrice,
			&r.ItemOrigin, &r.ItemEstWeightKg,
			&r.ItemPhotoPath, &r.Status, &r.ConvertedOrderID, &r.ConvertedCustomer, &r.CreatedAt,
			&r.FeeModel, &r.FeePercent, &r.FeePerKgIDR,
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
		return ErrAlreadyProcessed
	}
	return nil
}

// SetFee records the jastiper's chosen fee model + value on a pending request.
// Called by the accept handler before conversion.
func (s *Store) SetFee(ctx context.Context, jastiperID, id int64, model FeeModel, percent, perKgIDR *float64) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE buyer_requests
		SET fee_model = $1::fee_model, fee_percent = $2, fee_per_kg_idr = $3
		WHERE id = $4 AND jastiper_user_id = $5 AND status = 'pending'`,
		string(model), percent, perKgIDR, id, jastiperID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
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
			&r.BuyerNote, &r.ItemTitle, &r.ItemDescription, &r.ItemCurrency, &r.ItemEstPrice,
			&r.ItemOrigin, &r.ItemEstWeightKg,
			&r.ItemPhotoPath, &r.Status, &r.ConvertedOrderID, &r.ConvertedCustomer, &r.CreatedAt,
			&r.FeeModel, &r.FeePercent, &r.FeePerKgIDR,
			&r.JastiperName, &r.JastiperPhone); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
