package web

import (
	"bytes"
	"io"
	"net/http"

	"github.com/titipdong/titipdong/internal/scan"
)

func (s *Server) handleScanForm(w http.ResponseWriter, r *http.Request) {
	disabled := !s.cfg.HasOpenAI()
	s.render(w, r, "scan.html", map[string]any{"disabled": disabled})
}

// handleScan accepts a receipt photo, calls OpenAI vision, stashes the result
// in the session, and redirects to the order form with the fields pre-filled.
func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.HasOpenAI() {
		http.Error(w, "scan dinonaktifkan", http.StatusNotImplemented)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "form error", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		s.render(w, r, "scan.html", map[string]any{"error": "Upload foto struk dulu ya."})
		return
	}
	defer file.Close()

	imgBytes, err := io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	ct := header.Header.Get("Content-Type")
	result, err := s.scan.Extract(r.Context(), imgBytes, ct)
	if err != nil {
		s.render(w, r, "scan.html", map[string]any{"error": err.Error()})
		return
	}

	// Persist the photo so the merchant can attach it to the new order.
	var photoPath string
	if path, err := s.saveUpload(byteReader(imgBytes), header, "receipt"); err == nil {
		photoPath = path
	}
	result.PhotoPath = photoPath

	s.sessions.Put(r.Context(), "scanResult", result)
	http.Redirect(w, r, "/app/orders/new", http.StatusSeeOther)
}

// byteReader wraps a byte slice as an io.Reader for re-reading the upload.
func byteReader(b []byte) io.Reader { return bytes.NewReader(b) }

var _ = scan.Result{}
