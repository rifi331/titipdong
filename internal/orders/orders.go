// Package orders persists jastip orders and powers the status pipeline.
package orders

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Status is an order's stage in the pipeline:
// dicari -> ketemu -> dibeli -> dibayar -> diantar.
type Status string

const (
	StatusDicari  Status = "dicari"
	StatusKetemu  Status = "ketemu"
	StatusDibeli  Status = "dibeli"
	StatusDibayar Status = "dibayar"
	StatusDiantar Status = "diantar"
)

// Pipeline is the canonical, ordered sequence of statuses.
var Pipeline = []Status{StatusDicari, StatusKetemu, StatusDibeli, StatusDibayar, StatusDiantar}

// Order is a single item a jastiper is sourcing for a customer.
type Order struct {
	ID              int64
	OwnerUserID     int64
	CustomerID      *int64
	TripID          *int64
	ItemName        string
	SourceStore     string
	Currency        string
	AmountForeign   float64
	MarkupPct       float64
	FXRateSnapshot  float64
	SellingPriceIDR float64
	Status          Status
	Paid            bool
	PhotoPath       string
	Note            string
	CreatedAt       time.Time

	// Joined fields (optional, filled by queries that JOIN).
	CustomerName string
	TripName     string
}

// Store wraps the database.
type Store struct{ pool *pgxpool.Pool }

// NewStore constructs a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Create inserts a new order and returns it with id + computed selling price.
func (s *Store) Create(ctx context.Context, o Order) (Order, error) {
	if o.Status == "" {
		o.Status = StatusDicari
	}
	var out Order
	err := s.pool.QueryRow(ctx, `
		INSERT INTO orders
		  (owner_user_id, customer_id, trip_id, item_name, source_store, currency,
		   amount_foreign, markup_pct, fx_rate_snapshot, selling_price_idr,
		   status, paid, photo_path, note)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::order_status,$12,$13,$14)
		RETURNING id, owner_user_id, customer_id, trip_id, item_name, source_store, currency,
		          amount_foreign, markup_pct, fx_rate_snapshot, selling_price_idr,
		          status::text, paid, photo_path, note, created_at`,
		o.OwnerUserID, o.CustomerID, o.TripID, o.ItemName, o.SourceStore, o.Currency,
		o.AmountForeign, o.MarkupPct, o.FXRateSnapshot, o.SellingPriceIDR,
		string(o.Status), o.Paid, o.PhotoPath, o.Note).
		Scan(&out.ID, &out.OwnerUserID, &out.CustomerID, &out.TripID, &out.ItemName, &out.SourceStore, &out.Currency,
			&out.AmountForeign, &out.MarkupPct, &out.FXRateSnapshot, &out.SellingPriceIDR,
			&out.Status, &out.Paid, &out.PhotoPath, &out.Note, &out.CreatedAt)
	return out, err
}

// Get returns one order with customer/trip names if available.
func (s *Store) Get(ctx context.Context, ownerID, id int64) (Order, error) {
	var o Order
	var custName, tripName *string
	err := s.pool.QueryRow(ctx, `
		SELECT o.id, o.owner_user_id, o.customer_id, o.trip_id, o.item_name, o.source_store, o.currency,
		       o.amount_foreign, o.markup_pct, o.fx_rate_snapshot, o.selling_price_idr,
		       o.status::text, o.paid, o.photo_path, o.note, o.created_at,
		       c.name, t.name
		FROM orders o
		LEFT JOIN customers c ON c.id = o.customer_id
		LEFT JOIN trips t     ON t.id = o.trip_id
		WHERE o.id=$1 AND o.owner_user_id=$2`, id, ownerID).
		Scan(&o.ID, &o.OwnerUserID, &o.CustomerID, &o.TripID, &o.ItemName, &o.SourceStore, &o.Currency,
			&o.AmountForeign, &o.MarkupPct, &o.FXRateSnapshot, &o.SellingPriceIDR,
			&o.Status, &o.Paid, &o.PhotoPath, &o.Note, &o.CreatedAt,
			&custName, &tripName)
	if err != nil {
		return Order{}, err
	}
	if custName != nil {
		o.CustomerName = *custName
	}
	if tripName != nil {
		o.TripName = *tripName
	}
	return o, nil
}

// ListFilter controls which orders List returns.
type ListFilter struct {
	TripID     *int64
	Status     *Status
	OnlyUnpaid bool
}

// List returns orders for an owner, optionally filtered.
func (s *Store) List(ctx context.Context, ownerID int64, f ListFilter) ([]Order, error) {
	q := `
		SELECT o.id, o.owner_user_id, o.customer_id, o.trip_id, o.item_name, o.source_store, o.currency,
		       o.amount_foreign, o.markup_pct, o.fx_rate_snapshot, o.selling_price_idr,
		       o.status::text, o.paid, o.photo_path, o.note, o.created_at,
		       c.name, t.name
		FROM orders o
		LEFT JOIN customers c ON c.id = o.customer_id
		LEFT JOIN trips t     ON t.id = o.trip_id
		WHERE o.owner_user_id = $1`
	args := []any{ownerID}
	if f.TripID != nil {
		args = append(args, *f.TripID)
		q += fmtAppend(" AND o.trip_id = $%d", len(args))
	}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		q += fmtAppend(" AND o.status = $%d::order_status", len(args))
	}
	if f.OnlyUnpaid {
		q += " AND o.paid = FALSE"
	}
	q += " ORDER BY o.created_at DESC"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		var o Order
		var custName, tripName *string
		if err := rows.Scan(&o.ID, &o.OwnerUserID, &o.CustomerID, &o.TripID, &o.ItemName, &o.SourceStore, &o.Currency,
			&o.AmountForeign, &o.MarkupPct, &o.FXRateSnapshot, &o.SellingPriceIDR,
			&o.Status, &o.Paid, &o.PhotoPath, &o.Note, &o.CreatedAt,
			&custName, &tripName); err != nil {
			return nil, err
		}
		if custName != nil {
			o.CustomerName = *custName
		}
		if tripName != nil {
			o.TripName = *tripName
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Update edits an order's editable fields.
func (s *Store) Update(ctx context.Context, o Order) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE orders SET
		  customer_id=$1, trip_id=$2, item_name=$3, source_store=$4, currency=$5,
		  amount_foreign=$6, markup_pct=$7, fx_rate_snapshot=$8, selling_price_idr=$9,
		  photo_path=$10, note=$11
		WHERE id=$12 AND owner_user_id=$13`,
		o.CustomerID, o.TripID, o.ItemName, o.SourceStore, o.Currency,
		o.AmountForeign, o.MarkupPct, o.FXRateSnapshot, o.SellingPriceIDR,
		o.PhotoPath, o.Note, o.ID, o.OwnerUserID)
	return err
}

// SetStatus advances an order to the given status.
func (s *Store) SetStatus(ctx context.Context, ownerID, id int64, st Status) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE orders SET status=$1::order_status WHERE id=$2 AND owner_user_id=$3`,
		string(st), id, ownerID)
	return err
}

// SetPaid flips the paid flag.
func (s *Store) SetPaid(ctx context.Context, ownerID, id int64, paid bool) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE orders SET paid=$1 WHERE id=$2 AND owner_user_id=$3`, paid, id, ownerID)
	return err
}

// Delete removes an order.
func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM orders WHERE id=$1 AND owner_user_id=$2`, id, ownerID)
	return err
}

// NextStatus returns the next pipeline step after cur, or cur if already final.
func NextStatus(cur Status) Status {
	for i, s := range Pipeline {
		if s == cur && i+1 < len(Pipeline) {
			return Pipeline[i+1]
		}
	}
	return cur
}

// StatusLabel is the Bahasa Indonesia human label for a status.
func StatusLabel(st Status) string {
	switch st {
	case StatusDicari:
		return "Dicari"
	case StatusKetemu:
		return "Ketemu"
	case StatusDibeli:
		return "Dibeli"
	case StatusDibayar:
		return "Dibayar"
	case StatusDiantar:
		return "Diantar"
	}
	return string(st)
}

// StatusEmoji gives a quick at-a-glance glyph for the pipeline.
func StatusEmoji(st Status) string {
	switch st {
	case StatusDicari:
		return "🔍"
	case StatusKetemu:
		return "✅"
	case StatusDibeli:
		return "🛍️"
	case StatusDibayar:
		return "💰"
	case StatusDiantar:
		return "📦"
	}
	return ""
}

// Summary aggregates orders for the trip dashboard / end-of-trip summary.
type Summary struct {
	OrderCount     int
	RevenueIDR     float64 // sum of selling_price_idr
	CostIDR        float64 // sum of (amount_foreign * fx_rate_snapshot)
	NetMarginIDR   float64
	NetMarginPct   float64
	PaidCount      int
	UnpaidCount    int
	OutstandingIDR float64 // unpaid selling price
}

// Summarize computes totals across a set of orders (optionally trip-scoped).
func (s *Store) Summarize(ctx context.Context, ownerID int64, tripID *int64) (Summary, error) {
	q := `
		SELECT count(*),
		       COALESCE(SUM(selling_price_idr),0),
		       COALESCE(SUM(amount_foreign * fx_rate_snapshot),0),
		       count(*) FILTER (WHERE paid),
		       count(*) FILTER (WHERE NOT paid),
		       COALESCE(SUM(selling_price_idr) FILTER (WHERE NOT paid),0)
		FROM orders WHERE owner_user_id=$1`
	args := []any{ownerID}
	if tripID != nil {
		args = append(args, *tripID)
		q += fmtAppend(" AND trip_id = $%d", len(args))
	}
	var sum Summary
	err := s.pool.QueryRow(ctx, q, args...).Scan(
		&sum.OrderCount, &sum.RevenueIDR, &sum.CostIDR,
		&sum.PaidCount, &sum.UnpaidCount, &sum.OutstandingIDR)
	if err != nil {
		return Summary{}, err
	}
	sum.NetMarginIDR = sum.RevenueIDR - sum.CostIDR
	if sum.CostIDR > 0 {
		sum.NetMarginPct = sum.NetMarginIDR / sum.CostIDR * 100
	}
	return sum, nil
}

// CustomerBreakdown is per-customer totals for the dashboard.
type CustomerBreakdown struct {
	CustomerID   *int64
	CustomerName string
	OrderCount   int
	TotalIDR     float64
	PaidIDR      float64
	Outstanding  float64
}

// BreakdownByCustomer returns per-customer aggregates.
func (s *Store) BreakdownByCustomer(ctx context.Context, ownerID int64, tripID *int64) ([]CustomerBreakdown, error) {
	q := `
		SELECT o.customer_id, COALESCE(c.name,'(tanpa customer)'),
		       count(*) AS order_count,
		       COALESCE(SUM(o.selling_price_idr),0) AS total_idr,
		       COALESCE(SUM(o.selling_price_idr) FILTER (WHERE o.paid),0) AS paid_idr,
		       COALESCE(SUM(o.selling_price_idr) FILTER (WHERE NOT o.paid),0) AS outstanding
		FROM orders o
		LEFT JOIN customers c ON c.id = o.customer_id
		WHERE o.owner_user_id=$1`
	args := []any{ownerID}
	if tripID != nil {
		args = append(args, *tripID)
		q += fmtAppend(" AND o.trip_id = $%d", len(args))
	}
	q += " GROUP BY o.customer_id, c.name ORDER BY total_idr DESC"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CustomerBreakdown
	for rows.Next() {
		var b CustomerBreakdown
		if err := rows.Scan(&b.CustomerID, &b.CustomerName, &b.OrderCount, &b.TotalIDR, &b.PaidIDR, &b.Outstanding); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// CategoryRollup counts orders by source_store (used as best-store proxy).
type CategoryRollup struct {
	SourceStore string
	OrderCount  int
	RevenueIDR  float64
}

// TopStores returns the busiest source stores.
func (s *Store) TopStores(ctx context.Context, ownerID int64, tripID *int64, limit int) ([]CategoryRollup, error) {
	q := `
		SELECT COALESCE(NULLIF(source_store,''),'(unknown)') AS source_store,
		       count(*) AS order_count,
		       COALESCE(SUM(selling_price_idr),0) AS revenue_idr
		FROM orders WHERE owner_user_id=$1`
	args := []any{ownerID}
	if tripID != nil {
		args = append(args, *tripID)
		q += fmtAppend(" AND trip_id = $%d", len(args))
	}
	q += " GROUP BY source_store ORDER BY order_count DESC"
	if limit > 0 {
		q += fmtAppend(" LIMIT %d", limit)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CategoryRollup
	for rows.Next() {
		var c CategoryRollup
		if err := rows.Scan(&c.SourceStore, &c.OrderCount, &c.RevenueIDR); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// fmtAppend formats a SQL fragment with positional args (used for optional WHERE/LIMIT).
func fmtAppend(format string, a ...any) string {
	return fmt.Sprintf(format, a...)
}
