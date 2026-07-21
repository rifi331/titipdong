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
// in a single transaction. The jastiper sets the fee model (percent or per_kg)
// in the form; the order's selling price is computed accordingly. On commit,
// the request is marked accepted with back-references to the created rows.
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

	// Read the jastiper's fee choice from the form (defaults to percent 20).
	feeModel := requests.FeePercent
	if m := r.FormValue("fee_model"); m == "per_kg" {
		feeModel = requests.FeePerKg
	}
	var feePercent, feePerKgIDR *float64
	if feeModel == requests.FeePercent {
		p := parseAmount(r.FormValue("fee_percent"))
		if p == 0 {
			p = 20
		}
		feePercent = &p
	} else {
		p := parseAmount(r.FormValue("fee_per_kg_idr"))
		if p == 0 {
			p = 50000
		}
		feePerKgIDR = &p
	}

	// Convert: create customer + order in one tx, mark request accepted + fee set.
	orderID, customerID, err := s.convertRequestToOrder(r.Context(), u.ID, req, feeModel, feePercent, feePerKgIDR)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Record back-references (non-fatal if it fails — conversion already happened).
	_ = s.requests.MarkConverted(r.Context(), req.ID, orderID, customerID)

	// Build a WA link to notify the buyer. Selling price already computed on the order.
	estIDR := s.estimatedIDR(r.Context(), req.ItemEstPrice, req.ItemCurrency)
	waLink := whatsapp.JastiperToBuyerAcceptLink(
		req.BuyerWhatsApp, req.BuyerName, req.ItemTitle,
		req.ItemEstPrice, req.ItemCurrency, estIDR,
	)
	http.Redirect(w, r, waLink, http.StatusSeeOther)
}

// convertRequestToOrder creates the customer + order rows for an accepted
// request inside a single transaction. Returns (orderID, customerID).
// The fee model determines how selling_price_idr is computed:
//   - percent:  selling = (item_est_price × fx) × (1 + fee_percent/100)
//   - per_kg:   selling = item_est_weight_kg × fee_per_kg_idr
// For per_kg, amount_foreign/fx are kept as the item reference (not the fee).
func (s *Server) convertRequestToOrder(ctx context.Context, ownerID int64, req requests.Request,
	feeModel requests.FeeModel, feePercent, feePerKgIDR *float64) (int64, int64, error) {
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

	// 2. Compute selling price based on the jastiper's chosen fee model.
	//    - percent:  selling = (item_est_price × fx) × (1 + fee_percent/100)
	//                markup_pct column = fee_percent, amount_foreign = item price.
	//    - per_kg:   selling = item_est_weight_kg × fee_per_kg_idr
	//                markup_pct = 0, amount_foreign = 0 (fee isn't tied to item price).
	rate, _ := s.currency.Rate(ctx, req.ItemCurrency)
	if rate <= 0 {
		rate = 1
	}
	var markup, selling float64
	switch feeModel {
	case requests.FeePerKg:
		weight := req.ItemEstWeightKg
		perKg := 0.0
		if feePerKgIDR != nil {
			perKg = *feePerKgIDR
		}
		selling = weight * perKg
		markup = 0
		// Keep the item's reference price for display, but it's not part of selling.
	default: // FeePercent
		pct := 20.0
		if feePercent != nil {
			pct = *feePercent
		}
		markup = pct
		selling = rate * req.ItemEstPrice * (1 + pct/100.0)
	}

	var orderID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO orders
		  (owner_user_id, customer_id, item_name, source_store, currency,
		   amount_foreign, markup_pct, fx_rate_snapshot, selling_price_idr,
		   status, note)
		VALUES ($1, $2, $3, '', $4, $5, $6, $7, $8, 'accepted', $9)
		RETURNING id`,
		ownerID, customerID, req.ItemTitle, req.ItemCurrency,
		req.ItemEstPrice, markup, rate, selling,
		"dari request #"+itoa(req.ID),
	).Scan(&orderID)
	if err != nil {
		return 0, 0, err
	}

	// 2b. Stamp the request with the chosen fee model + values.
	if _, err := tx.Exec(ctx, `
		UPDATE buyer_requests
		SET fee_model = $1::fee_model, fee_percent = $2, fee_per_kg_idr = $3
		WHERE id = $4`,
		string(feeModel), feePercent, feePerKgIDR, req.ID); err != nil {
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
	estIDR := s.estimatedIDR(r.Context(), req.ItemEstPrice, req.ItemCurrency)
	waLink := whatsapp.JastiperToBuyerAcceptLink(
		req.BuyerWhatsApp, req.BuyerName, req.ItemTitle,
		req.ItemEstPrice, req.ItemCurrency, estIDR,
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
