package web

import (
	"net/http"
)

// handleCatalogItemDetail shows a single catalog item with photo, description,
// jastiper info, and action buttons (Mau Ini / Request Custom).
func (s *Server) handleCatalogItemDetail(w http.ResponseWriter, r *http.Request) {
	id := pathInt64(r, "id")
	item, err := s.catalog.GetPublic(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "catalog_item_detail.html", map[string]any{"item": item})
}
