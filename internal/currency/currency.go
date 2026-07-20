// Package currency converts foreign amounts to IDR using a live FX API with DB caching.
package currency

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Supported lists currencies the jastiper is likely to encounter.
var Supported = []string{"JPY", "KRW", "SGD", "THB", "HKD", "USD", "CNY", "TWD", "MYR", "AUD", "EUR", "GBP", "IDR", "PHP", "INR", "CAD", "NZD", "CHF"}

// Service fetches and caches FX rates.
type Service struct {
	pool    *pgxpool.Pool
	baseURL string
	client  *http.Client
	mu      sync.Mutex
}

// New constructs a Service.
func New(pool *pgxpool.Pool, baseURL string) *Service {
	return &Service{
		pool:    pool,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 8 * time.Second},
	}
}

// frankfurterResponse models frankfurter.app/latest?from=X&to=IDR.
type frankfurterResponse struct {
	Rates map[string]float64 `json:"rates"`
}

// Rate returns units of IDR per 1 unit of `code`. Falls back to DB cache on error.
// For IDR itself returns 1.
func (s *Service) Rate(ctx context.Context, code string) (float64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || code == "IDR" {
		return 1, nil
	}

	// 1. Try DB cache (< 24h old).
	if r, ok, err := s.cached(ctx, code); err == nil && ok {
		return r, nil
	}

	// 2. Fetch fresh.
	rate, err := s.fetch(ctx, code)
	if err != nil {
		// Last resort: stale cache.
		if r, ok := s.cachedStale(ctx, code); ok {
			return r, nil
		}
		return 0, err
	}

	_ = s.store(ctx, code, rate)
	return rate, nil
}

func (s *Service) fetch(ctx context.Context, code string) (float64, error) {
	if s.baseURL == "" {
		return 0, fmt.Errorf("FX_BASE_URL not set")
	}
	url := fmt.Sprintf("%s/latest?from=%s&to=IDR", s.baseURL, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fx fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("fx %s: HTTP %d: %s", code, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var fr frankfurterResponse
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		return 0, err
	}
	r, ok := fr.Rates["IDR"]
	if !ok || r <= 0 {
		return 0, fmt.Errorf("fx %s: missing IDR rate", code)
	}
	return r, nil
}

func (s *Service) cached(ctx context.Context, code string) (float64, bool, error) {
	var rate float64
	var fetched time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT rate, fetched_at FROM fx_rates WHERE base=$1 AND quote='IDR'`, code).
		Scan(&rate, &fetched)
	if err != nil {
		return 0, false, nil // missing -> not an error here
	}
	if time.Since(fetched) > 24*time.Hour {
		return 0, false, nil
	}
	return rate, true, nil
}

func (s *Service) cachedStale(ctx context.Context, code string) (float64, bool) {
	var rate float64
	err := s.pool.QueryRow(ctx,
		`SELECT rate FROM fx_rates WHERE base=$1 AND quote='IDR'`, code).Scan(&rate)
	if err != nil {
		return 0, false
	}
	return rate, true
}

func (s *Service) store(ctx context.Context, code string, rate float64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO fx_rates (base, quote, rate, fetched_at)
		VALUES ($1, 'IDR', $2, now())
		ON CONFLICT (base, quote) DO UPDATE SET rate = EXCLUDED.rate, fetched_at = now()`,
		code, rate)
	return err
}

// ToIDR converts a foreign amount to IDR using the given rate.
func ToIDR(amountForeign, rate float64) float64 {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return 0
	}
	return amountForeign * rate
}

// SellingPrice applies markup and rounds: foreign -> IDR -> +markup%.
func SellingPrice(amountForeign, rate, markupPct float64) float64 {
	base := ToIDR(amountForeign, rate)
	return base * (1 + markupPct/100.0)
}
