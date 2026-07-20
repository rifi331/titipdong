package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// pathInt64 reads an :id path param.
func pathInt64(r *http.Request, key string) int64 {
	v := chi.URLParam(r, key)
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

// parseAmount parses "1.800" / "1800" / "1,800.50" loosely to a float.
// Indonesian convention uses dot as thousands sep; we tolerate both.
func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Keep only digits, separators, sign.
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' || r == '-' {
			b.WriteRune(r)
		}
	}
	s = b.String()
	if s == "" {
		return 0
	}
	switch {
	case strings.Contains(s, ".") && strings.Contains(s, ","):
		// Both present: the last one is the decimal separator.
		if strings.LastIndex(s, ",") > strings.LastIndex(s, ".") {
			// 1.800,50 -> 1800.50
			s = strings.ReplaceAll(s, ".", "")
			s = strings.ReplaceAll(s, ",", ".")
		} else {
			// 1,800.50 -> 1800.50
			s = strings.ReplaceAll(s, ",", "")
		}
	case strings.Contains(s, ","):
		// Single comma -> decimal.
		s = strings.ReplaceAll(s, ",", ".")
	case strings.Count(s, ".") > 1:
		// Multiple dots => thousands separators only.
		s = strings.ReplaceAll(s, ".", "")
	case strings.Contains(s, "."):
		// Single dot. Treat as thousands separator if it groups exactly 3 digits
		// at the end (Indonesian convention "1.800"); otherwise as a decimal
		// point ("1.8" or "1800.5").
		if idx := strings.Index(s, "."); idx >= 0 && len(s)-idx-1 == 3 && !strings.ContainsAny(s, "-") {
			s = strings.ReplaceAll(s, ".", "")
		}
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parseOptionalInt returns a *int64 for nullable references (customer/trip).
func parseOptionalInt(s string) *int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n == 0 {
		return nil
	}
	return &n
}

// saveUpload persists a multipart file to the uploads dir and returns its relative filename.
func (s *Server) saveUpload(file io.Reader, header *multipart.FileHeader, prefix string) (string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = guessExt(header)
	}
	if !isAllowedExt(ext) {
		return "", fmt.Errorf("tipe file tidak didukung: %s", ext)
	}
	name := prefix + "_" + randHex(8) + ext
	if err := os.MkdirAll(s.cfg.UploadsDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(s.cfg.UploadsDir, name)
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}
	return name, nil
}

func guessExt(fh *multipart.FileHeader) string {
	exts, _ := mime.ExtensionsByType(fh.Header.Get("Content-Type"))
	if len(exts) > 0 {
		return exts[0]
	}
	return ".jpg"
}

func isAllowedExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".heic":
		return true
	}
	return false
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
