// Package orders persists jastip orders and powers the status lifecycle.
//
// Status flow (v2 — single source of truth, no separate `paid` boolean):
//
//	pending_confirmation -> accepted -> waiting_for_payment -> paid -> delivery -> finished
//	             |             |              |               |
//	         rejected    seller_cancelled  seller_cancelled   (terminal)
//	                      buyer_cancelled
//
// Payment detail (paid_at, payment_method, paid_amount, payment_ref) is filled
// when status transitions to `paid`, for audit / dispute resolution.
package orders

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Status is an order's stage in the lifecycle.
type Status string

const (
	StatusPendingConfirmation Status = "pending_confirmation"
	StatusAccepted            Status = "accepted"
	StatusRejected            Status = "rejected"
	StatusBuyerCancelled      Status = "buyer_cancelled"
	StatusWaitingForPayment   Status = "waiting_for_payment"
	StatusSellerCancelled     Status = "seller_cancelled"
	StatusPaid                Status = "paid"
	StatusDelivery            Status = "delivery"
	StatusFinished            Status = "finished"
)

// TerminalStatuses are the statuses from which no further action is expected.
var TerminalStatuses = []Status{StatusFinished, StatusRejected, StatusBuyerCancelled, StatusSellerCancelled}

// PaidStatuses are statuses where payment has been received (revenue counts).
var PaidStatuses = []Status{StatusPaid, StatusDelivery, StatusFinished}

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
	PhotoPath       string
	Note            string
	CreatedAt       time.Time

	// Payment detail — filled when status transitions to `paid`.
	PaidAt        *time.Time
	PaymentMethod string
	PaidAmount    *float64
	PaymentRef    string

	// Joined fields (optional, filled by queries that JOIN).
	CustomerName string
	TripName     string
	OwnerName    string // jastiper display name (admin views)
}

// Store wraps the database.
type Store struct{ pool *pgxpool.Pool }

// NewStore constructs a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// orderColumns is the canonical column list for SELECT statements.
const orderColumns = `
	o.id, o.owner_user_id, o.customer_id, o.trip_id, o.item_name, o.source_store,
	o.currency, o.amount_foreign, o.markup_pct, o.fx_rate_snapshot, o.selling_price_idr,
	o.status::text, o.photo_path, o.note, o.created_at,
	o.paid_at, o.payment_method, o.paid_amount, o.payment_ref,
	COALESCE(c.name, ''), COALESCE(t.name, ''), COALESCE(u.display_name, u.email)`

const orderJoins = `
	FROM orders o
	LEFT JOIN customers c ON c.id = o.customer_id
	LEFT JOIN trips t     ON t.id = o.trip_id
	LEFT JOIN users u     ON u.id = o.owner_user_id`

// scanOrder scans one row into an Order (works for both QueryRow and Rows).
func scanOrder(row interface {
	Scan(dest ...any) error
}) (Order, error) {
	var o Order
	var custName, tripName, ownerName string
	err := row.Scan(&o.ID, &o.OwnerUserID, &o.CustomerID, &o.TripID, &o.ItemName, &o.SourceStore,
		&o.Currency, &o.AmountForeign, &o.MarkupPct, &o.FXRateSnapshot, &o.SellingPriceIDR,
		&o.Status, &o.PhotoPath, &o.Note, &o.CreatedAt,
		&o.PaidAt, &o.PaymentMethod, &o.PaidAmount, &o.PaymentRef,
		&custName, &tripName, &ownerName)
	o.CustomerName = custName
	o.TripName = tripName
	o.OwnerName = ownerName
	return o, err
}

// Create inserts a new order and returns it with id + computed selling price.
func (s *Store) Create(ctx context.Context, o Order) (Order, error) {
	if o.Status == "" {
		o.Status = StatusAccepted
	}
	var out Order
	err := s.pool.QueryRow(ctx, `
		INSERT INTO orders
		  (owner_user_id, customer_id, trip_id, item_name, source_store, currency,
		   amount_foreign, markup_pct, fx_rate_snapshot, selling_price_idr,
		   status, photo_path, note)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::order_status,$12,$13)
		RETURNING id, owner_user_id, customer_id, trip_id, item_name, source_store, currency,
		          amount_foreign, markup_pct, fx_rate_snapshot, selling_price_idr,
		          status::text, photo_path, note, created_at,
		          paid_at, payment_method, paid_amount, payment_ref`,
		o.OwnerUserID, o.CustomerID, o.TripID, o.ItemName, o.SourceStore, o.Currency,
		o.AmountForeign, o.MarkupPct, o.FXRateSnapshot, o.SellingPriceIDR,
		string(o.Status), o.PhotoPath, o.Note).
		Scan(&out.ID, &out.OwnerUserID, &out.CustomerID, &out.TripID, &out.ItemName, &out.SourceStore, &out.Currency,
			&out.AmountForeign, &out.MarkupPct, &out.FXRateSnapshot, &out.SellingPriceIDR,
			&out.Status, &out.PhotoPath, &out.Note, &out.CreatedAt,
			&out.PaidAt, &out.PaymentMethod, &out.PaidAmount, &out.PaymentRef)
	return out, err
}

// Get returns one order with customer/trip/owner names if available.
func (s *Store) Get(ctx context.Context, ownerID, id int64) (Order, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+orderColumns+orderJoins+` WHERE o.id=$1 AND o.owner_user_id=$2`, id, ownerID)
	o, err := scanOrder(row)
	return o, err
}

// GetByID returns one order by id only (admin scope — no owner filter).
func (s *Store) GetByID(ctx context.Context, id int64) (Order, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+orderColumns+orderJoins+` WHERE o.id=$1`, id)
	o, err := scanOrder(row)
	return o, err
}

// ListFilter controls which orders List returns.
type ListFilter struct {
	TripID          *int64
	Status          *Status
	OnlyOutstanding bool // status = 'waiting_for_payment'
}

// List returns orders for an owner, optionally filtered.
func (s *Store) List(ctx context.Context, ownerID int64, f ListFilter) ([]Order, error) {
	q := `SELECT ` + orderColumns + orderJoins + ` WHERE o.owner_user_id = $1`
	args := []any{ownerID}
	if f.TripID != nil {
		args = append(args, *f.TripID)
		q += fmt.Sprintf(" AND o.trip_id = $%d", len(args))
	}
	if f.Status != nil {
		args = append(args, string(*f.Status))
		q += fmt.Sprintf(" AND o.status = $%d::order_status", len(args))
	}
	if f.OnlyOutstanding {
		q += " AND o.status = 'waiting_for_payment'"
	}
	q += " ORDER BY o.created_at DESC"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ListAll returns orders across all jastipers (admin view).
// Optional filters by status and owner.
func (s *Store) ListAll(ctx context.Context, status *Status, ownerID *int64) ([]Order, error) {
	q := `SELECT ` + orderColumns + orderJoins + ` WHERE 1=1`
	args := []any{}
	if status != nil {
		args = append(args, string(*status))
		q += fmt.Sprintf(" AND o.status = $%d::order_status", len(args))
	}
	if ownerID != nil {
		args = append(args, *ownerID)
		q += fmt.Sprintf(" AND o.owner_user_id = $%d", len(args))
	}
	q += " ORDER BY o.created_at DESC"
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Update edits an order's editable fields (excludes status + payment detail).
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

// SetStatus changes an order's status. No transition validation (any -> any).
func (s *Store) SetStatus(ctx context.Context, ownerID, id int64, st Status) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE orders SET status=$1::order_status WHERE id=$2 AND owner_user_id=$3`,
		string(st), id, ownerID)
	return err
}

// MarkPaid transitions an order to `paid` and records the payment detail.
// amount defaults to selling_price_idr if 0. method/ref are optional.
func (s *Store) MarkPaid(ctx context.Context, ownerID, id int64, method, ref string, amount float64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE orders
		SET status='paid',
		    paid_at = now(),
		    payment_method = $1,
		    payment_ref = $2,
		    paid_amount = CASE WHEN $3 > 0 THEN $3 ELSE selling_price_idr END
		WHERE id=$4 AND owner_user_id=$5`,
		method, ref, amount, id, ownerID)
	return err
}

// Delete removes an order.
func (s *Store) Delete(ctx context.Context, ownerID, id int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM orders WHERE id=$1 AND owner_user_id=$2`, id, ownerID)
	return err
}

// StatusLabel is the Bahasa Indonesia human label for a status.
func StatusLabel(st Status) string {
	switch st {
	case StatusPendingConfirmation:
		return "Menunggu Konfirmasi"
	case StatusAccepted:
		return "Diterima"
	case StatusRejected:
		return "Ditolak"
	case StatusBuyerCancelled:
		return "Dibatalkan Buyer"
	case StatusWaitingForPayment:
		return "Menunggu Pembayaran"
	case StatusSellerCancelled:
		return "Dibatalkan Jastiper"
	case StatusPaid:
		return "Lunas"
	case StatusDelivery:
		return "Diantar"
	case StatusFinished:
		return "Selesai"
	}
	return string(st)
}

// StatusEmoji gives a quick at-a-glance glyph for a status.
func StatusEmoji(st Status) string {
	switch st {
	case StatusPendingConfirmation:
		return "⏳"
	case StatusAccepted:
		return "✅"
	case StatusRejected:
		return "❌"
	case StatusBuyerCancelled, StatusSellerCancelled:
		return "🚫"
	case StatusWaitingForPayment:
		return "💵"
	case StatusPaid:
		return "💰"
	case StatusDelivery:
		return "📦"
	case StatusFinished:
		return "🎉"
	}
	return ""
}

// IsPaid reports whether the status counts as paid (revenue realized).
func IsPaid(st Status) bool {
	for _, p := range PaidStatuses {
		if p == st {
			return true
		}
	}
	return false
}

// Summary aggregates orders for the trip dashboard / end-of-trip summary.
type Summary struct {
	OrderCount       int
	RevenueIDR       float64 // sum of selling_price_idr for paid statuses
	CostIDR          float64 // sum of (amount_foreign * fx_rate_snapshot)
	NetMarginIDR     float64
	NetMarginPct     float64
	PaidCount        int
	OutstandingIDR   float64 // sum of selling_price for waiting_for_payment
	OutstandingCount int
}

// Summarize computes totals across a set of orders (optionally trip-scoped).
// Excludes rejected/cancelled orders from revenue + cost.
func (s *Store) Summarize(ctx context.Context, ownerID int64, tripID *int64) (Summary, error) {
	q := `
		SELECT count(*),
		       COALESCE(SUM(selling_price_idr) FILTER (WHERE status IN ('paid','delivery','finished')),0),
		       COALESCE(SUM(amount_foreign * fx_rate_snapshot),0),
		       count(*) FILTER (WHERE status IN ('paid','delivery','finished')),
		       COALESCE(SUM(selling_price_idr) FILTER (WHERE status='waiting_for_payment'),0),
		       count(*) FILTER (WHERE status='waiting_for_payment')
		FROM orders
		WHERE owner_user_id=$1 AND status NOT IN ('rejected','buyer_cancelled','seller_cancelled')`
	args := []any{ownerID}
	if tripID != nil {
		args = append(args, *tripID)
		q += fmt.Sprintf(" AND trip_id = $%d", len(args))
	}
	var sum Summary
	err := s.pool.QueryRow(ctx, q, args...).Scan(
		&sum.OrderCount, &sum.RevenueIDR, &sum.CostIDR,
		&sum.PaidCount, &sum.OutstandingIDR, &sum.OutstandingCount)
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
		       COALESCE(SUM(o.selling_price_idr) FILTER (WHERE o.status IN ('paid','delivery','finished')),0) AS paid_idr,
		       COALESCE(SUM(o.selling_price_idr) FILTER (WHERE o.status='waiting_for_payment'),0) AS outstanding,
		       COALESCE(SUM(o.selling_price_idr),0) AS total_idr
		FROM orders o
		LEFT JOIN customers c ON c.id = o.customer_id
		WHERE o.owner_user_id=$1 AND o.status NOT IN ('rejected','buyer_cancelled','seller_cancelled')`
	args := []any{ownerID}
	if tripID != nil {
		args = append(args, *tripID)
		q += fmt.Sprintf(" AND o.trip_id = $%d", len(args))
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
		if err := rows.Scan(&b.CustomerID, &b.CustomerName, &b.OrderCount, &b.PaidIDR, &b.Outstanding, &b.TotalIDR); err != nil {
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
		       COALESCE(SUM(selling_price_idr) FILTER (WHERE status IN ('paid','delivery','finished')),0) AS revenue_idr
		FROM orders
		WHERE owner_user_id=$1 AND status NOT IN ('rejected','buyer_cancelled','seller_cancelled')`
	args := []any{ownerID}
	if tripID != nil {
		args = append(args, *tripID)
		q += fmt.Sprintf(" AND trip_id = $%d", len(args))
	}
	q += " GROUP BY source_store ORDER BY order_count DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
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
