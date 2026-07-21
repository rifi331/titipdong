package web

import (
	"net/http"

	"github.com/titipdong/titipdong/internal/auth"
	"github.com/titipdong/titipdong/internal/whatsapp"
)

// handleOrderMessage returns the WhatsApp-style update message as plain text,
// so the jastiper can copy it to the clipboard and paste into any chat app
// (Instagram DM, Telegram, SMS, etc.) when the buyer has no WhatsApp.
//
// Example: GET /app/orders/5/message -> "Halo Bu Yuni, Hada Labo ketemu, ..."
func (s *Server) handleOrderMessage(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	o, err := s.orders.Get(r.Context(), u.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	custName := o.CustomerName
	if custName == "" {
		custName = "Kak"
	}
	msg := whatsapp.Message(custName, o.ItemName, o.Status, o.SellingPriceIDR)
	if msg == "" {
		msg = "(tidak ada pesan untuk status ini)"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(msg))
}
