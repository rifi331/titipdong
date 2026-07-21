package web

import (
	"net/http"
	"strconv"

	"github.com/titipdong/titipdong/internal/orders"
)

// handleAdminOrders lists all orders across jastipers (read-only admin view).
// Filters: ?status=paid, ?jastiper=5.
func (s *Server) handleAdminOrders(w http.ResponseWriter, r *http.Request) {
	var status *orders.Status
	if v := r.URL.Query().Get("status"); v != "" {
		st := orders.Status(v)
		status = &st
	}
	var ownerID *int64
	if v := r.URL.Query().Get("jastiper"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			ownerID = &n
		}
	}
	list, err := s.orders.ListAll(r.Context(), status, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Fetch jastiper list for filter dropdown.
	jastipers, _ := s.pool.Query(r.Context(),
		`SELECT id, COALESCE(display_name, email) FROM users WHERE role IN ('jastiper','admin') ORDER BY display_name`)
	defer jastipers.Close()
	type jrow struct{ ID int64; Name string }
	var jas []jrow
	for jastipers.Next() {
		var j jrow
		_ = jastipers.Scan(&j.ID, &j.Name)
		jas = append(jas, j)
	}
	s.render(w, r, "admin_orders.html", map[string]any{
		"orders":      list,
		"jastipers":   jas,
		"statusFilter": r.URL.Query().Get("status"),
		"jastiperFilter": r.URL.Query().Get("jastiper"),
	})
}
