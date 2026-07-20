package web

import (
	"net/http"
	"strings"

	"github.com/titipdong/titipdong/internal/auth"
	"github.com/titipdong/titipdong/internal/catalog"
)

// handleCatalogPublic is the public landing catalog (no login required).
func (s *Server) handleCatalogPublic(w http.ResponseWriter, r *http.Request) {
	items, _ := s.catalog.ListPublic(r.Context())
	s.render(w, r, "catalog_public.html", map[string]any{"items": items})
}

// handleCatalog is the authenticated view: jastiper sees their own items on top,
// plus the public list; buyer sees the public list.
func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	data := map[string]any{}
	public, _ := s.catalog.ListPublic(r.Context())
	data["public"] = public
	if u.Role == auth.RoleJastiper || u.Role == auth.RoleAdmin {
		mine, _ := s.catalog.ListByOwner(r.Context(), u.ID)
		data["mine"] = mine
	}
	s.render(w, r, "catalog.html", data)
}

func (s *Server) handleCatalogCreate(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	if u.Role != auth.RoleJastiper && u.Role != auth.RoleAdmin {
		http.Error(w, "hanya jastiper", http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		_ = r.ParseForm()
	}
	f := r.Form
	it := catalog.Item{
		JastiperUserID:  u.ID,
		Title:           strings.TrimSpace(f.Get("title")),
		Description:     strings.TrimSpace(f.Get("description")),
		EstPriceForeign: parseAmount(f.Get("est_price_foreign")),
		Currency:        strings.ToUpper(strings.TrimSpace(f.Get("currency"))),
	}
	if it.Currency == "" {
		it.Currency = "JPY"
	}
	if file, header, err := r.FormFile("photo"); err == nil {
		defer file.Close()
		if p, err := s.saveUpload(file, header, "catalog"); err == nil {
			it.PhotoPath = p
		}
	}
	if it.Title == "" {
		http.Redirect(w, r, "/app/catalog", http.StatusSeeOther)
		return
	}
	_, _ = s.catalog.Create(r.Context(), it)
	http.Redirect(w, r, "/app/catalog", http.StatusSeeOther)
}

func (s *Server) handleCatalogDelete(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	_ = s.catalog.Delete(r.Context(), u.ID, pathInt64(r, "id"))
	http.Redirect(w, r, "/app/catalog", http.StatusSeeOther)
}
