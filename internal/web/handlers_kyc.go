package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/titipdong/titipdong/internal/auth"
	"github.com/titipdong/titipdong/internal/kyc"
)

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	latest, has, _ := s.kyc.LatestForUser(r.Context(), u.ID)
	s.render(w, r, "profile.html", map[string]any{
		"app":    latest,
		"hasApp": has,
	})
}

func (s *Server) handleBecomeJastiper(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	if u.Role != auth.RoleBuyer {
		http.Error(w, "Sudah jadi jastiper/admin.", http.StatusBadRequest)
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "form error", http.StatusBadRequest)
		return
	}
	fullName := strings.TrimSpace(r.FormValue("full_name"))
	ktpNumber := strings.TrimSpace(r.FormValue("ktp_number"))
	phone := strings.TrimSpace(r.FormValue("phone"))
	city := strings.TrimSpace(r.FormValue("city"))
	if fullName == "" || ktpNumber == "" {
		s.render(w, r, "profile.html", map[string]any{
			"error": "Nama lengkap dan nomor KTP wajib diisi.",
		}, http.StatusBadRequest)
		return
	}

	// Optional KTP photo.
	var photoPath string
	file, header, err := r.FormFile("ktp_photo")
	if err == nil {
		defer file.Close()
		path, err := s.saveUpload(file, header, "ktp")
		if err != nil {
			s.render(w, r, "profile.html", map[string]any{"error": err.Error()}, http.StatusBadRequest)
			return
		}
		photoPath = path
	}

	// Block re-submit if already pending.
	if existing, has, _ := s.kyc.LatestForUser(r.Context(), u.ID); has && existing.Status == kyc.StatusPending {
		http.Redirect(w, r, "/app/profile", http.StatusSeeOther)
		return
	}

	_, err = s.kyc.Submit(r.Context(), kyc.Application{
		UserID:       u.ID,
		FullName:     fullName,
		KTPNumber:    ktpNumber,
		KTPPhotoPath: photoPath,
		Phone:        phone,
		City:         city,
	})
	if err != nil {
		s.render(w, r, "profile.html", map[string]any{"error": err.Error()}, http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app/profile", http.StatusSeeOther)
}

// --- Admin: users + applications ---

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.pool.Query(r.Context(), `
		SELECT id, email, role::text, display_name, created_at
		FROM users ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	type urow struct {
		ID          int64
		Email       string
		Role        string
		DisplayName string
		CreatedAt   time.Time
	}
	var users []urow
	for rows.Next() {
		var u urow
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &u.DisplayName, &u.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users = append(users, u)
	}
	s.render(w, r, "admin_users.html", map[string]any{"users": users})
}

func (s *Server) handleAdminSetRole(w http.ResponseWriter, r *http.Request) {
	role := r.FormValue("role")
	var target auth.Role
	switch role {
	case "buyer", "jastiper", "admin":
		target = auth.Role(role)
	default:
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	id := pathInt64(r, "id")
	if err := s.auth.SetRole(r.Context(), id, target); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app/admin/users", http.StatusSeeOther)
}

func (s *Server) handleAdminApplications(w http.ResponseWriter, r *http.Request) {
	which := r.URL.Query().Get("status")
	var apps []kyc.Application
	var err error
	if which == "pending" || which == "" {
		apps, err = s.kyc.ListPending(r.Context())
	} else {
		apps, err = s.kyc.ListAll(r.Context())
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, "admin_applications.html", map[string]any{"apps": apps, "filter": which})
}

func (s *Server) handleAdminApprove(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	if err := s.kyc.Decide(r.Context(), id, u.ID, true, ""); err != nil {
		if errors.Is(err, kyc.ErrAlreadyProcessed) {
			// Idempotent: application already approved/rejected, just go back to list.
			http.Redirect(w, r, "/app/admin/applications", http.StatusSeeOther)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app/admin/applications", http.StatusSeeOther)
}

func (s *Server) handleAdminReject(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.UserFrom(r)
	id := pathInt64(r, "id")
	note := strings.TrimSpace(r.FormValue("note"))
	if err := s.kyc.Decide(r.Context(), id, u.ID, false, note); err != nil {
		if errors.Is(err, kyc.ErrAlreadyProcessed) {
			http.Redirect(w, r, "/app/admin/applications", http.StatusSeeOther)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/app/admin/applications", http.StatusSeeOther)
}
