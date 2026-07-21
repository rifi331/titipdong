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

// SupportedCodes returns the same list as Supported (named accessor for
// callers that don't want to depend on the slice field directly).
func SupportedCodes() []string { return Supported }

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

// Rate returns units of IDR per 1 unit of `code`. Priority:
//  1. DB cache that is NOT stale (< 24h old) — the jastiper-controlled source
//     of truth; admin's "refresh rates" button keeps this fresh on demand.
//  2. Live Frankfurter fetch (when the DB row is missing OR stale) — auto-
//     cached into the DB so subsequent calls hit step 1.
//  3. Last-resort stale DB cache — better than returning 0 if Frankfurter is
//     down; the jastiper can still edit the rate manually in the order form.
//  4. Zero on total failure (no DB row, Frankfurter down).
func (s *Service) Rate(ctx context.Context, code string) (float64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" || code == "IDR" {
		return 1, nil
	}

	// 1. DB cache, fresh (< 24h).
	if r, ok := s.cached(ctx, code); ok {
		return r, nil
	}

	// 2. Missing or stale -> fetch from Frankfurter and cache.
	rate, err := s.fetch(ctx, code)
	if err != nil {
		// 3. Frankfurter failed -> fall back to whatever's in the DB (even stale).
		if r, ok := s.cachedStale(ctx, code); ok {
			return r, nil
		}
		return 0, err
	}
	_ = s.store(ctx, code, rate)
	return rate, nil
}

// RefreshAll fetches fresh rates for every supported currency from Frankfurter
// and stores them into the DB. Returns the per-currency outcome (rate or error).
// Intended for the admin "refresh rates" button.
func (s *Service) RefreshAll(ctx context.Context) map[string]RefreshResult {
	out := make(map[string]RefreshResult, len(Supported))
	for _, code := range Supported {
		if code == "IDR" {
			continue
		}
		rate, err := s.fetch(ctx, code)
		if err != nil {
			out[code] = RefreshResult{Err: err}
			continue
		}
		if err := s.store(ctx, code, rate); err != nil {
			out[code] = RefreshResult{Err: err}
			continue
		}
		out[code] = RefreshResult{Rate: rate}
	}
	return out
}

// RefreshResult is the per-currency outcome of RefreshAll.
type RefreshResult struct {
	Rate float64
	Err  error
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

// cached returns the DB rate only if it is fresh (< 24h old).
func (s *Service) cached(ctx context.Context, code string) (float64, bool) {
	var rate float64
	var fetched time.Time
	err := s.pool.QueryRow(ctx,
		`SELECT rate, fetched_at FROM fx_rates WHERE base=$1 AND quote='IDR'`, code).
		Scan(&rate, &fetched)
	if err != nil {
		return 0, false
	}
	if time.Since(fetched) > 24*time.Hour {
		return 0, false
	}
	return rate, true
}

// cachedStale returns whatever rate is stored in the DB, regardless of age.
// Used as last-resort fallback when Frankfurter is down.
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
