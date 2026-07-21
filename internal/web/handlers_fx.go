package web

import (
	"net/http"
	"strconv"
)

// handleFXRate returns the cached/live IDR rate for a currency as plain text.
// Used by the order form's Alpine.js live price preview.
// Example: GET /app/orders/fx?currency=MYR -> "4386.13"
func (s *Server) handleFXRate(w http.ResponseWriter, r *http.Request) {
	cur := r.URL.Query().Get("currency")
	if cur == "" {
		cur = "JPY"
	}
	rate, err := s.currency.Rate(r.Context(), cur)
	if err != nil || rate <= 0 {
		rate = 0
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write([]byte(strconv.FormatFloat(rate, 'f', 2, 64)))
}
