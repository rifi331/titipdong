package web

import (
	"net/http"
	"strings"

	"github.com/titipdong/titipdong/internal/requests"
	"github.com/titipdong/titipdong/internal/whatsapp"
)

// handleCustomRequestForm renders the public custom-item request form.
// Buyer picks a jastiper, describes the item (name, origin, est price, est weight).
func (s *Server) handleCustomRequestForm(w http.ResponseWriter, r *http.Request) {
	// Optional: pre-select a jastiper via ?jastiper=<id>
	selectedJastiper := r.URL.Query().Get("jastiper")
	jastipers, err := s.listJastipers(r)
	if err != nil || len(jastipers) == 0 {
		s.render(w, r, "custom_request_form.html", map[string]any{
			"error":   "Belum ada jastiper yang tersedia. Coba lagi nanti ya!",
			"hasNone": true,
		})
		return
	}
	s.render(w, r, "custom_request_form.html", map[string]any{
		"jastipers":        jastipers,
		"selectedJastiper": selectedJastiper,
	})
}

// jastiperRow is a minimal display struct for the jastiper picker.
type jastiperRow struct {
	ID, Name, Phone, UpcomingTrip string
}

func (s *Server) listJastipers(r *http.Request) ([]jastiperRow, error) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT u.id::text, COALESCE(u.display_name, u.email),
		       COALESCE((SELECT phone FROM jastiper_applications a
		                 WHERE a.user_id = u.id AND a.status='approved'
		                 ORDER BY created_at DESC LIMIT 1), ''),
		       COALESCE((SELECT t.name || ' (' || t.country || ')'
		                 FROM trips t
		                 WHERE t.owner_user_id = u.id AND t.status = 'active'
		                 ORDER BY t.start_date DESC NULLS LAST LIMIT 1), '')
		FROM users u
		WHERE u.role = 'jastiper'
		ORDER BY u.display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []jastiperRow
	for rows.Next() {
		var j jastiperRow
		if err := rows.Scan(&j.ID, &j.Name, &j.Phone, &j.UpcomingTrip); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// handleCustomRequestSubmit accepts a custom-item request without a catalog id.
func (s *Server) handleCustomRequestSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form error", http.StatusBadRequest)
		return
	}
	jastiperID := parseOptionalInt(r.FormValue("jastiper_id"))
	name := strings.TrimSpace(r.FormValue("buyer_name"))
	wa := whatsappDigits(r.FormValue("buyer_whatsapp"))
	itemTitle := strings.TrimSpace(r.FormValue("item_title"))
	itemOrigin := strings.TrimSpace(r.FormValue("item_origin"))
	itemCurrency := strings.ToUpper(strings.TrimSpace(r.FormValue("item_currency")))
	itemEstPrice := parseAmount(r.FormValue("item_est_price"))
	itemWeight := parseAmount(r.FormValue("item_est_weight_kg"))
	note := strings.TrimSpace(r.FormValue("buyer_note"))

	if name == "" || wa == "" || itemTitle == "" || jastiperID == nil {
		jastipers, _ := s.listJastipers(r)
		s.render(w, r, "custom_request_form.html", map[string]any{
			"jastipers": jastipers,
			"error":     "Nama, WhatsApp, nama barang, dan jastiper wajib diisi.",
			"form": map[string]any{
				"jastiper_id": r.FormValue("jastiper_id"),
				"buyer_name":  name, "buyer_whatsapp": r.FormValue("buyer_whatsapp"),
				"item_title": itemTitle, "item_origin": itemOrigin,
				"item_currency": itemCurrency, "item_est_price": r.FormValue("item_est_price"),
				"item_est_weight_kg": r.FormValue("item_est_weight_kg"), "buyer_note": note,
			},
		}, http.StatusBadRequest)
		return
	}
	if itemCurrency == "" {
		itemCurrency = "JPY"
	}

	// Lookup the jastiper's display name + phone for the WA link.
	var jastiperName, jastiperPhone string
	_ = s.pool.QueryRow(r.Context(), `
		SELECT COALESCE(display_name, email),
		       COALESCE((SELECT phone FROM jastiper_applications a
		                 WHERE a.user_id = $1 AND a.status='approved'
		                 ORDER BY created_at DESC LIMIT 1), '')
		FROM users WHERE id = $1 AND role = 'jastiper'`, *jastiperID).Scan(&jastiperName, &jastiperPhone)
	if jastiperName == "" {
		jastipers, _ := s.listJastipers(r)
		s.render(w, r, "custom_request_form.html", map[string]any{
			"jastipers": jastipers,
			"error":     "Jastiper yang kamu pilih tidak ditemukan.",
			"form": map[string]any{
				"buyer_name": name, "buyer_whatsapp": r.FormValue("buyer_whatsapp"),
				"item_title": itemTitle, "item_origin": itemOrigin,
				"item_currency": itemCurrency, "item_est_price": r.FormValue("item_est_price"),
				"item_est_weight_kg": r.FormValue("item_est_weight_kg"), "buyer_note": note,
			},
		}, http.StatusBadRequest)
		return
	}

	_, err := s.requests.Submit(r.Context(), requests.Request{
		JastiperUserID:  *jastiperID,
		BuyerName:       name,
		BuyerWhatsApp:   wa,
		BuyerNote:       note,
		ItemTitle:       itemTitle,
		ItemCurrency:    itemCurrency,
		ItemEstPrice:    itemEstPrice,
		ItemOrigin:      itemOrigin,
		ItemEstWeightKg: itemWeight,
	})
	if err != nil {
		jastipers, _ := s.listJastipers(r)
		s.render(w, r, "custom_request_form.html", map[string]any{
			"jastipers": jastipers,
			"error":     "Gagal kirim request. Coba lagi sebentar ya.",
			"form": map[string]any{
				"jastiper_id": *jastiperID, "buyer_name": name,
				"buyer_whatsapp": r.FormValue("buyer_whatsapp"),
				"item_title": itemTitle, "item_origin": itemOrigin,
				"item_currency": itemCurrency, "item_est_price": r.FormValue("item_est_price"),
				"item_est_weight_kg": r.FormValue("item_est_weight_kg"), "buyer_note": note,
			},
		}, http.StatusInternalServerError)
		return
	}

	waLink := ""
	if jastiperPhone != "" {
		waLink = whatsapp.BuyerRequestToJastiperLink(jastiperPhone, jastiperName, name, itemTitle, note)
	}
	s.sessions.Put(r.Context(), "thanksWALink", waLink)
	s.sessions.Put(r.Context(), "thanksItem", itemTitle)
	s.sessions.Put(r.Context(), "thanksJastiper", jastiperName)
	http.Redirect(w, r, "/catalog/thanks", http.StatusSeeOther)
}
