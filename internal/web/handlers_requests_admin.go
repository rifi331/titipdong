package web

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/titipdong/titipdong/internal/auth"
	"github.com/titipdong/titipdong/internal/requests"
	"github.com/titipdong/titipdong/internal/whatsapp"
)

// handleRequestsList renders the jastiper's request queue.
func (s *Server) handleRequestsList(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	pending, err := s.requests.ListPending(r.Context(), u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	all, err := s.requests.ListForJastiper(r.Context(), u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// "recent" = last 10 non-pending (accepted/rejected history).
	var recent []requests.Request
	for _, req := range all {
		if req.Status != requests.StatusPending {
			recent = append(recent, req)
			if len(recent) >= 10 {
				break
			}
		}
	}
	s.render(w, r, "requests_dashboard.html", map[string]any{
		"pending": pending,
		"recent":  recent,
	})
}

// handleRequestAccept converts a pending request into a real order + customer
// in a single transaction. On commit, the request is marked accepted with
// back-references to the created rows.
func (s *Server) handleRequestAccept(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	req, err := s.requests.Get(r.Context(), u.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if req.Status != requests.StatusPending {
		http.Redirect(w, r, "/app/requests", http.StatusSeeOther)
		return
	}

	// Convert: create customer + order in one tx, mark request accepted.
	orderID, customerID, err := s.convertRequestToOrder(r.Context(), u.ID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Record back-references (non-fatal if it fails — conversion already happened).
	_ = s.requests.MarkConverted(r.Context(), req.ID, orderID, customerID)

	// Build a WA link to notify the buyer.
	estIDR := s.estimatedIDR(r.Context(), req.ItemEstPriceForeign, req.ItemCurrency)
	waLink := whatsapp.JastiperToBuyerAcceptLink(
		req.BuyerWhatsApp, req.BuyerName, req.ItemTitle,
		req.ItemEstPriceForeign, req.ItemCurrency, estIDR,
	)
	http.Redirect(w, r, waLink, http.StatusSeeOther)
}

// convertRequestToOrder creates the customer + order rows for an accepted
// request inside a single transaction. Returns (orderID, customerID).
func (s *Server) convertRequestToOrder(ctx context.Context, ownerID int64, req requests.Request) (int64, int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)

	// 1. Create a customer for this buyer (deduplicated later by the jastiper
	//    if they recognize an existing one; here we just create a fresh row).
	var customerID int64
	custName := req.BuyerName
	if n := strings.TrimSpace(custName); n == "" {
		custName = "Buyer (dari katalog)"
	}
	custNotes := "dari request katalog #" + itoa(req.ID)
	if n := strings.TrimSpace(req.BuyerNote); n != "" {
		custNotes += "; note: " + n
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO customers (owner_user_id, name, whatsapp, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		ownerID, custName, req.BuyerWhatsApp, custNotes,
	).Scan(&customerID)
	if err != nil {
		return 0, 0, err
	}

	// 2. Snapshot FX + compute selling price (20% default markup; jastiper
	//    can edit the order afterwards to adjust).
	rate, _ := s.currency.Rate(ctx, req.ItemCurrency)
	if rate <= 0 {
		rate = 1
	}
	markup := 20.0
	selling := rate * req.ItemEstPriceForeign * (1 + markup/100.0)

	var orderID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO orders
		  (owner_user_id, customer_id, item_name, source_store, currency,
		   amount_foreign, markup_pct, fx_rate_snapshot, selling_price_idr,
		   status, note)
		VALUES ($1, $2, $3, '', $4, $5, $6, $7, $8, 'dicari', $9)
		RETURNING id`,
		ownerID, customerID, req.ItemTitle, req.ItemCurrency,
		req.ItemEstPriceForeign, markup, rate, selling,
		"dari request katalog #"+itoa(req.ID),
	).Scan(&orderID)
	if err != nil {
		return 0, 0, err
	}

	// 3. Mark the request accepted (idempotent within tx).
	tag, err := tx.Exec(ctx,
		`UPDATE buyer_requests SET status='accepted' WHERE id=$1 AND jastiper_user_id=$2 AND status='pending'`,
		req.ID, ownerID)
	if err != nil {
		return 0, 0, err
	}
	if tag.RowsAffected() == 0 {
		return 0, 0, requests.ErrAlreadyProcessed
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return orderID, customerID, nil
}

// $ifelse returns a if cond else b — helper used inline above.
// (Removed — replaced by explicit custNotes string construction above.)

// handleRequestReject marks a request rejected. Optional note is stashed in
// session for the WA link that follows.
func (s *Server) handleRequestReject(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	note := strings.TrimSpace(r.FormValue("note"))
	req, err := s.requests.Get(r.Context(), u.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.requests.SetStatus(r.Context(), u.ID, id, requests.StatusRejected); err != nil {
		if errors.Is(err, requests.ErrAlreadyProcessed) {
			http.Redirect(w, r, "/app/requests", http.StatusSeeOther)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Optional WA to buyer.
	if req.BuyerWhatsApp != "" {
		waLink := whatsapp.JastiperToBuyerRejectLink(req.BuyerWhatsApp, req.BuyerName, req.ItemTitle, note)
		http.Redirect(w, r, waLink, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/app/requests", http.StatusSeeOther)
}

// handleRequestWA redirects to a wa.me link to message the buyer.
func (s *Server) handleRequestWA(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	req, err := s.requests.Get(r.Context(), u.ID, id)
	if err != nil || req.BuyerWhatsApp == "" {
		http.Error(w, "nomor WhatsApp buyer gak tersedia", http.StatusBadRequest)
		return
	}
	estIDR := s.estimatedIDR(r.Context(), req.ItemEstPriceForeign, req.ItemCurrency)
	waLink := whatsapp.JastiperToBuyerAcceptLink(
		req.BuyerWhatsApp, req.BuyerName, req.ItemTitle,
		req.ItemEstPriceForeign, req.ItemCurrency, estIDR,
	)
	http.Redirect(w, r, waLink, http.StatusSeeOther)
}

// estimatedIDR converts a foreign price to IDR (best-effort, for WA message).
func (s *Server) estimatedIDR(ctx context.Context, amountForeign float64, currency string) float64 {
	rate, err := s.currency.Rate(ctx, currency)
	if err != nil || rate <= 0 {
		return 0
	}
	return rate * amountForeign
}
