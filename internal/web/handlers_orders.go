package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/titipdong/titipdong/internal/auth"
	"github.com/titipdong/titipdong/internal/orders"
	"github.com/titipdong/titipdong/internal/whatsapp"
)

// listFilters builds a ListFilter from query params.
func listFilters(r *http.Request) orders.ListFilter {
	var f orders.ListFilter
	if v := r.URL.Query().Get("trip"); v != "" {
		id := pathInt64Like(v)
		if id > 0 {
			f.TripID = &id
		}
	}
	if v := r.URL.Query().Get("status"); v != "" {
		st := orders.Status(v)
		f.Status = &st
	}
	if r.URL.Query().Get("outstanding") == "1" {
		f.OnlyOutstanding = true
	}
	return f
}

// pathInt64Like parses a plain string id (not a chi path param).
func pathInt64Like(s string) int64 {
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int64(r-'0')
	}
	return n
}

func (s *Server) handleOrdersList(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	f := listFilters(r)
	list, err := s.orders.List(r.Context(), u.ID, f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	trips, _ := s.trips.List(r.Context(), u.ID)
	var activeTripName string
	if f.TripID != nil {
		for _, t := range trips {
			if t.ID == *f.TripID {
				activeTripName = t.Name
			}
		}
	}
	s.render(w, r, "orders.html", map[string]any{
		"orders":         list,
		"trips":          trips,
		"filter":         f,
		"activeTripName": activeTripName,
	})
}

func (s *Server) handleOrderNew(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	custs, _ := s.customers.List(r.Context(), u.ID)
	trips, _ := s.trips.List(r.Context(), u.ID)
	s.render(w, r, "order_form.html", map[string]any{
		"customers":  custs,
		"trips":      trips,
		"scanResult": s.popScanResult(r),
	})
}

// popScanResult returns a scan result stashed in the session, then clears it.
func (s *Server) popScanResult(r *http.Request) any {
	v := s.sessions.Pop(r.Context(), "scanResult")
	return v
}

func (s *Server) handleOrderCreate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	o, err := s.orderFromForm(r, u.ID, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	created, err := s.orders.Create(r.Context(), o)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.sessions.Put(r.Context(), "flash", "✅ Order \""+created.ItemName+"\" berhasil dibuat!")
	http.Redirect(w, r, "/app/orders/"+itoa(created.ID), http.StatusSeeOther)
}

func (s *Server) handleOrderEdit(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	o, err := s.orders.Get(r.Context(), u.ID, pathInt64(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	custs, _ := s.customers.List(r.Context(), u.ID)
	trips, _ := s.trips.List(r.Context(), u.ID)
	flash, _ := s.sessions.Pop(r.Context(), "flash").(string)
	s.render(w, r, "order_form.html", map[string]any{
		"order":     o,
		"customers": custs,
		"trips":     trips,
		"flash":     flash,
	})
}

func (s *Server) handleOrderUpdate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	o, err := s.orderFromForm(r, u.ID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	o.ID = id
	if err := s.orders.Update(r.Context(), o); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app/orders/"+itoa(id), http.StatusSeeOther)
}

func (s *Server) handleOrderDelete(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	_ = s.orders.Delete(r.Context(), u.ID, pathInt64(r, "id"))
	http.Redirect(w, r, "/app/orders", http.StatusSeeOther)
}

// handleOrderStatusChange transitions an order to a new status.
// Replaces the old handleOrderAdvance + handleOrderTogglePaid.
// Form fields: status (required), and when status=paid:
// payment_method, payment_ref, paid_amount (all optional).
// HTMX-friendly: returns the updated order card partial.
func (s *Server) handleOrderStatusChange(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	target := orders.Status(r.FormValue("status"))
	if !isValidStatus(target) {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	// Route to MarkPaid if target is 'paid' (records payment detail).
	if target == orders.StatusPaid {
		method := strings.TrimSpace(r.FormValue("payment_method"))
		ref := strings.TrimSpace(r.FormValue("payment_ref"))
		amount := parseAmount(r.FormValue("paid_amount"))
		if err := s.orders.MarkPaid(r.Context(), u.ID, id, method, ref, amount); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if err := s.orders.SetStatus(r.Context(), u.ID, id, target); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if r.Header.Get("HX-Request") == "true" {
		o, _ := s.orders.Get(r.Context(), u.ID, id)
		waLink := s.waLinkForOrder(r.Context(), o)
		s.renderPartial(w, "order_card.html", map[string]any{"o": o, "waLink": waLink})
		return
	}
	http.Redirect(w, r, "/app/orders/"+itoa(id), http.StatusSeeOther)
}

// isValidStatus reports whether s is one of the 9 known statuses.
func isValidStatus(s orders.Status) bool {
	switch s {
	case orders.StatusPendingConfirmation, orders.StatusAccepted, orders.StatusRejected,
		orders.StatusBuyerCancelled, orders.StatusWaitingForPayment, orders.StatusSellerCancelled,
		orders.StatusPaid, orders.StatusDelivery, orders.StatusFinished:
		return true
	}
	return false
}

// handleOrderWhatsApp redirects to a wa.me deep link for the order.
func (s *Server) handleOrderWhatsApp(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	o, err := s.orders.Get(r.Context(), u.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if o.CustomerID == nil {
		http.Error(w, "order tidak punya customer", http.StatusBadRequest)
		return
	}
	cust, err := s.customers.Get(r.Context(), u.ID, *o.CustomerID)
	if err != nil || cust.WhatsApp == "" {
		http.Error(w, "customer tidak punya nomor WhatsApp", http.StatusBadRequest)
		return
	}
	link := s.waLinkForOrder(r.Context(), o)
	if link == "" {
		http.Error(w, "tidak ada pesan WA untuk status ini", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, link, http.StatusSeeOther)
}

// waLinkForOrder builds the wa.me URL using the order's customer + status.
// Returns "" if no customer, no WhatsApp number, or the status has no
// customer-facing message (finished, cancelled).
func (s *Server) waLinkForOrder(ctx context.Context, o orders.Order) string {
	if o.CustomerID == nil {
		return ""
	}
	cust, err := s.customers.Get(ctx, o.OwnerUserID, *o.CustomerID)
	if err != nil || cust.WhatsApp == "" {
		return ""
	}
	// Status is the single source of truth now — no paid workaround.
	msg := whatsapp.Message(cust.Name, o.ItemName, o.Status, o.SellingPriceIDR)
	if msg == "" {
		return "" // no message for terminal/cancelled statuses
	}
	return whatsapp.ComposeLink(cust.WhatsApp, cust.Name, o.ItemName, o.Status, o.SellingPriceIDR)
}

// orderFromForm parses form fields (including photo upload) into an Order,
// computes the FX rate snapshot and selling price.
func (s *Server) orderFromForm(r *http.Request, ownerID, id int64) (orders.Order, error) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		// Fall back to plain form (no upload).
		_ = r.ParseForm()
	}
	f := r.Form
	o := orders.Order{
		OwnerUserID:   ownerID,
		ItemName:      strings.TrimSpace(f.Get("item_name")),
		SourceStore:   strings.TrimSpace(f.Get("source_store")),
		Currency:      strings.ToUpper(strings.TrimSpace(f.Get("currency"))),
		AmountForeign: parseAmount(f.Get("amount_foreign")),
		MarkupPct:     parseAmount(f.Get("markup_pct")),
		Note:          strings.TrimSpace(f.Get("note")),
	}
	if o.Currency == "" {
		o.Currency = "JPY"
	}
	o.CustomerID = parseOptionalInt(f.Get("customer_id"))
	o.TripID = parseOptionalInt(f.Get("trip_id"))

	// Snapshot FX rate at order time. The jastiper can override the cached
	// rate manually (24h cache can be stale; they may have a fresher rate
	// from their own money-changer). If the override is empty/0, fall back
	// to the cached/live rate from the FX service.
	rate := parseAmount(f.Get("fx_rate_override"))
	if rate <= 0 {
		fetched, err := s.currency.Rate(r.Context(), o.Currency)
		if err != nil {
			rate = 1 // graceful fallback; jastiper can correct later
		} else {
			rate = fetched
		}
	}
	o.FXRateSnapshot = rate
	o.SellingPriceIDR = sellingPrice(o.AmountForeign, rate, o.MarkupPct)

	// Optional item photo upload.
	if file, header, err := r.FormFile("photo"); err == nil {
		defer file.Close()
		path, err := s.saveUpload(file, header, "item")
		if err == nil {
			o.PhotoPath = path
		}
	} else if existing := strings.TrimSpace(f.Get("existing_photo")); existing != "" {
		o.PhotoPath = existing
	}
	return o, nil
}

func sellingPrice(amountForeign, rate, markupPct float64) float64 {
	return rate * amountForeign * (1 + markupPct/100)
}

// itoa is a tiny int64->string helper to avoid strconv imports in some files.
func itoa(n int64) string {
	return formatInt64(n)
}

func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}
