package web

import (
	"net/http"
	"strings"

	"github.com/titipdong/titipdong/internal/requests"
	"github.com/titipdong/titipdong/internal/whatsapp"
)

// handleRequestForm renders the public "Mau Ini!" form for a catalog item.
// No login required — this is the buyer-side entry point.
func (s *Server) handleRequestForm(w http.ResponseWriter, r *http.Request) {
	id := pathInt64(r, "id")
	item, err := s.catalog.GetPublic(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "request_form.html", map[string]any{"item": item})
}

// handleRequestSubmit accepts the anonymous buyer's name + WhatsApp + note,
// saves the request, then redirects to a thanks page that offers a wa.me link
// to the jastiper (so the buyer can tap Send to notify them instantly).
func (s *Server) handleRequestSubmit(w http.ResponseWriter, r *http.Request) {
	id := pathInt64(r, "id")
	item, err := s.catalog.GetPublic(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form error", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("buyer_name"))
	wa := whatsappDigits(r.FormValue("buyer_whatsapp"))
	note := strings.TrimSpace(r.FormValue("buyer_note"))
	if name == "" || wa == "" {
		s.render(w, r, "request_form.html", map[string]any{
			"item":  item,
			"error": "Nama dan nomor WhatsApp wajib diisi.",
			"name":  name,
			"wa":    r.FormValue("buyer_whatsapp"),
			"note":  note,
		}, http.StatusBadRequest)
		return
	}

	// Lookup the jastiper's WhatsApp (from their most recent approved KYC).
	jastiperPhone := s.kyc.PhoneForUser(r.Context(), item.JastiperUserID)

	// Persist the request.
	_, err = s.requests.Submit(r.Context(), requests.Request{
		CatalogItemID:  item.ID,
		JastiperUserID: item.JastiperUserID,
		BuyerName:      name,
		BuyerWhatsApp:  wa,
		BuyerNote:      note,
	})
	if err != nil {
		// Most likely the spam-throttle unique index fired.
		s.render(w, r, "request_form.html", map[string]any{
			"item":  item,
			"error": "Requestmu baru saja terkirim untuk item ini. Tunggu sebentar ya, jastiper akan hubungi kamu.",
			"name":  name, "wa": r.FormValue("buyer_whatsapp"), "note": note,
		}, http.StatusTooManyRequests)
		return
	}

	// Build the wa.me link to the jastiper (buyer taps Send to notify).
	waLink := ""
	if jastiperPhone != "" {
		waLink = whatsapp.BuyerRequestToJastiperLink(jastiperPhone, item.JastiperName, name, item.Title, note)
	}

	// Stash for the thanks page.
	s.sessions.Put(r.Context(), "thanksWALink", waLink)
	s.sessions.Put(r.Context(), "thanksItem", item.Title)
	s.sessions.Put(r.Context(), "thanksJastiper", item.JastiperName)
	http.Redirect(w, r, "/catalog/thanks", http.StatusSeeOther)
}

// handleRequestThanks is the landing page after submit.
func (s *Server) handleRequestThanks(w http.ResponseWriter, r *http.Request) {
	waLink, _ := s.sessions.Pop(r.Context(), "thanksWALink").(string)
	itemTitle, _ := s.sessions.Pop(r.Context(), "thanksItem").(string)
	jastiperName, _ := s.sessions.Pop(r.Context(), "thanksJastiper").(string)

	s.render(w, r, "request_thanks.html", map[string]any{
		"waLink":       waLink,
		"itemTitle":    itemTitle,
		"jastiperName": jastiperName,
	})
}
