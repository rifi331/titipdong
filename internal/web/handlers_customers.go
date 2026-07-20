package web

import (
	"net/http"
	"strings"

	"github.com/titipdong/titipdong/internal/auth"
	"github.com/titipdong/titipdong/internal/customers"
)

func (s *Server) handleCustomersList(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	list, err := s.customers.List(r.Context(), u.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "customers.html", map[string]any{"customers": list})
}

func (s *Server) handleCustomerCreate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form error", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Redirect(w, r, "/app/customers", http.StatusSeeOther)
		return
	}
	_, _ = s.customers.Create(r.Context(), u.ID,
		name,
		whatsappDigits(r.FormValue("whatsapp")),
		strings.TrimSpace(r.FormValue("notes")),
	)
	http.Redirect(w, r, "/app/customers", http.StatusSeeOther)
}

func (s *Server) handleCustomerEdit(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	c, err := s.customers.Get(r.Context(), u.ID, pathInt64(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "customer_edit.html", map[string]any{"customer": c})
}

func (s *Server) handleCustomerUpdate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form error", http.StatusBadRequest)
		return
	}
	_ = s.customers.Update(r.Context(), u.ID, id,
		strings.TrimSpace(r.FormValue("name")),
		whatsappDigits(r.FormValue("whatsapp")),
		strings.TrimSpace(r.FormValue("notes")),
	)
	http.Redirect(w, r, "/app/customers", http.StatusSeeOther)
}

func (s *Server) handleCustomerDelete(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	_ = s.customers.Delete(r.Context(), u.ID, pathInt64(r, "id"))
	http.Redirect(w, r, "/app/customers", http.StatusSeeOther)
}

// whatsappDigits keeps only digits from a phone input.
func whatsappDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var _ = customers.Customer{}
