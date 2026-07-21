package web

import (
	"net/http"
	"time"

	"github.com/titipdong/titipdong/internal/currency"
)

// fxRow is a display row for the admin rates page.
type fxRow struct {
	Code, Rate, FetchedAge string
	Stale                  bool
}

// handleAdminRates shows the current FX rates cached in the DB.
func (s *Server) handleAdminRates(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT base, rate, fetched_at FROM fx_rates
		WHERE quote='IDR' ORDER BY base`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var list []fxRow
	for rows.Next() {
		var code string
		var rate float64
		var fetched time.Time
		if err := rows.Scan(&code, &rate, &fetched); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		age := time.Since(fetched)
		list = append(list, fxRow{
			Code:      code,
			Rate:      formatFloat(rate),
			FetchedAge: humanDuration(age),
			Stale:      age > 24*time.Hour,
		})
	}
	s.render(w, r, "admin_rates.html", map[string]any{"rates": list})
}

// handleAdminRefreshRates fetches fresh rates for all supported currencies
// from Frankfurter and stores them in the DB.
func (s *Server) handleAdminRefreshRates(w http.ResponseWriter, r *http.Request) {
	results := s.currency.RefreshAll(r.Context())
	// Build a flat list of "code: rate (ok)" or "code: error" for the page.
	type row struct {
		Code, Result string
		OK           bool
	}
	var summary []row
	for _, code := range currency.SupportedCodes() {
		if code == "IDR" {
			continue
		}
		rr, exists := results[code]
		if !exists {
			continue
		}
		if rr.Err != nil {
			summary = append(summary, row{Code: code, Result: rr.Err.Error(), OK: false})
		} else {
			summary = append(summary, row{Code: code, Result: formatFloat(rr.Rate), OK: true})
		}
	}
	// Re-render the rates page with the refresh summary at the top.
	rows, _ := s.pool.Query(r.Context(),
		`SELECT base, rate, fetched_at FROM fx_rates WHERE quote='IDR' ORDER BY base`)
	defer rows.Close()
	var list []fxRow
	for rows.Next() {
		var code string
		var rate float64
		var fetched time.Time
		_ = rows.Scan(&code, &rate, &fetched)
		age := time.Since(fetched)
		list = append(list, fxRow{
			Code: code, Rate: formatFloat(rate),
			FetchedAge: humanDuration(age), Stale: age > 24*time.Hour,
		})
	}
	s.render(w, r, "admin_rates.html", map[string]any{
		"rates":    list,
		"refreshed": summary,
	})
}

// formatFloat formats a float with up to 4 decimals, no trailing zeros.
func formatFloat(v float64) string {
	return floatToString(v, 4)
}

// humanDuration renders a duration in a friendly "3h ago" / "2d ago" form.
func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "baru saja"
	case d < time.Hour:
		return formatInt64(int64(d.Minutes())) + " menit lalu"
	case d < 24*time.Hour:
		return formatInt64(int64(d.Hours())) + " jam lalu"
	default:
		return formatInt64(int64(d.Hours()/24)) + " hari lalu"
	}
}
