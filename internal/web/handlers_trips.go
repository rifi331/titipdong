package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/titipdong/titipdong/internal/auth"
	"github.com/titipdong/titipdong/internal/orders"
	"github.com/titipdong/titipdong/internal/trips"
	"github.com/titipdong/titipdong/internal/whatsapp"
)

func (s *Server) handleTripsList(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	list, _ := s.trips.List(r.Context(), u.ID)
	s.render(w, r, "trips.html", map[string]any{"trips": list})
}

func (s *Server) handleTripCreate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form error", http.StatusBadRequest)
		return
	}
	t := trips.Trip{
		OwnerUserID: u.ID,
		Name:        strings.TrimSpace(r.FormValue("name")),
		Country:     strings.TrimSpace(r.FormValue("country")),
		Currency:    strings.ToUpper(strings.TrimSpace(r.FormValue("currency"))),
	}
	if t.Name == "" {
		t.Name = "Trip " + time.Now().Format("Jan 2006")
	}
	t.StartDate = parseDatePtr(r.FormValue("start_date"))
	t.EndDate = parseDatePtr(r.FormValue("end_date"))
	if _, err := s.trips.Create(r.Context(), t); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app/trips", http.StatusSeeOther)
}

// handleTripDashboard shows totals + per-customer breakdown for a trip.
func (s *Server) handleTripDashboard(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	t, err := s.trips.Get(r.Context(), u.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sum, _ := s.orders.Summarize(r.Context(), u.ID, &id)
	breakdown, _ := s.orders.BreakdownByCustomer(r.Context(), u.ID, &id)
	topStores, _ := s.orders.TopStores(r.Context(), u.ID, &id, 5)

	// Top customer (by total IDR).
	var topCustomer string
	if len(breakdown) > 0 {
		topCustomer = breakdown[0].CustomerName
	}

	// Best "category" proxy = busiest source store.
	var bestStore string
	if len(topStores) > 0 {
		bestStore = topStores[0].SourceStore
	}

	summaryLink := waSummaryLink(t.Name, sum, topCustomer)

	s.render(w, r, "trip_dashboard.html", map[string]any{
		"trip":        t,
		"summary":     sum,
		"breakdown":   breakdown,
		"topStores":   topStores,
		"topCustomer": topCustomer,
		"bestStore":   bestStore,
		"summaryLink": summaryLink,
	})
}

func (s *Server) handleTripClose(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	_ = s.trips.SetStatus(r.Context(), u.ID, pathInt64(r, "id"), trips.StatusClosed)
	http.Redirect(w, r, "/app/trips", http.StatusSeeOther)
}

func parseDatePtr(s string) *time.Time {
	for _, layout := range []string{"2006-01-02", "02/01/2006", "1/2/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			tt := t
			return &tt
		}
	}
	return nil
}

// waSummaryLink builds a shareable WhatsApp link for the end-of-trip summary.
func waSummaryLink(tripName string, sum orders.Summary, topCustomer string) string {
	return whatsapp.TripSummaryShareLink(tripName, sum.OrderCount, sum.RevenueIDR, sum.NetMarginIDR, topCustomer)
}
