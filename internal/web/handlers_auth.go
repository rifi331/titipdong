package web

import (
	"net/http"

	"github.com/titipdong/titipdong/internal/auth"
)

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserFrom(r); ok {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	s.render(w, r, "home.html", map[string]any{})
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "login.html", map[string]any{})
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form error", http.StatusBadRequest)
		return
	}
	email := r.FormValue("email")
	password := r.FormValue("password")
	next := r.FormValue("next")
	if next == "" {
		next = "/app"
	}
	if err := s.auth.Login(r.Context(), w, r, email, password); err != nil {
		s.render(w, r, "login.html", map[string]any{"error": "Email atau password salah.", "email": email}, http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (s *Server) handleSignupGet(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "signup.html", map[string]any{})
}

func (s *Server) handleSignupPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "form error", http.StatusBadRequest)
		return
	}
	email := r.FormValue("email")
	password := r.FormValue("password")
	name := r.FormValue("name")
	if len(password) < 8 {
		s.render(w, r, "signup.html", map[string]any{
			"error": "Password minimal 8 karakter.",
			"email": email, "name": name,
		}, http.StatusBadRequest)
		return
	}
	u, err := s.auth.Create(r.Context(), email, password, name)
	if err != nil {
		s.render(w, r, "signup.html", map[string]any{
			"error": "Email sudah dipakai atau input tidak valid.",
			"email": email, "name": name,
		}, http.StatusBadRequest)
		return
	}
	// Auto-login after signup.
	if err := s.auth.Login(r.Context(), w, r, u.Email, password); err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	_ = s.auth.Logout(r.Context(), w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) handleAppHome(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	switch u.Role {
	case auth.RoleAdmin:
		// Admin: show pending applications count + quick links.
		pending, _ := s.kyc.ListPending(r.Context())
		s.render(w, r, "home_admin.html", map[string]any{"pending": pending})
	case auth.RoleJastiper:
		// Jastiper: orders overview + active trip.
		sum, _ := s.orders.Summarize(r.Context(), u.ID, nil)
		trips, _ := s.trips.List(r.Context(), u.ID)
		s.render(w, r, "home_jastiper.html", map[string]any{"summary": sum, "trips": trips})
	default:
		// Buyer: catalog.
		items, _ := s.catalog.ListPublic(r.Context())
		s.render(w, r, "home_buyer.html", map[string]any{"items": items})
	}
}
