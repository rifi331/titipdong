package web

import (
	"net/http"

	"github.com/titipdong/titipdong/internal/auth"
)

// handlePayments shows outstanding + paid per customer.
func (s *Server) handlePayments(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	breakdown, err := s.orders.BreakdownByCustomer(r.Context(), u.ID, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	sum, _ := s.orders.Summarize(r.Context(), u.ID, nil)
	s.render(w, r, "payments.html", map[string]any{
		"breakdown": breakdown,
		"summary":   sum,
	})
}

// handleSummary is the end-of-trip summary chooser (across all trips or a selected one).
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	trips, _ := s.trips.List(r.Context(), u.ID)
	sum, _ := s.orders.Summarize(r.Context(), u.ID, nil)
	breakdown, _ := s.orders.BreakdownByCustomer(r.Context(), u.ID, nil)
	topStores, _ := s.orders.TopStores(r.Context(), u.ID, nil, 5)

	var topCustomer string
	if len(breakdown) > 0 {
		topCustomer = breakdown[0].CustomerName
	}
	var bestStore string
	if len(topStores) > 0 {
		bestStore = topStores[0].SourceStore
	}

	summaryLink := waSummaryLink("Semua Trip", sum, topCustomer)

	s.render(w, r, "summary.html", map[string]any{
		"trips":       trips,
		"summary":     sum,
		"breakdown":   breakdown,
		"topStores":   topStores,
		"topCustomer": topCustomer,
		"bestStore":   bestStore,
		"summaryLink": summaryLink,
	})
}
