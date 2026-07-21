// Package web wires HTTP handlers, templates, and routes for the TitipDong server.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/titipdong/titipdong/internal/auth"
	"github.com/titipdong/titipdong/internal/currency"
	"github.com/titipdong/titipdong/internal/orders"
)

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

// templateFuncs are the helpers available in every template.
var templateFuncs = template.FuncMap{
	"fmtIDR":        fmtIDR,
	"fmtIDRShort":   fmtIDRShort,
	"fmtMoney":      fmtMoney,
	"fmtForeign":    fmtForeign,
	"fmt2":          fmt2,
	"statusLabel":   func(s orders.Status) string { return orders.StatusLabel(s) },
	"statusEmoji":   func(s orders.Status) string { return orders.StatusEmoji(s) },
	"nextLabel":     func(s orders.Status) string { return orders.StatusLabel(orders.NextStatus(s)) },
	"isFinal":       func(s orders.Status) bool { return orders.NextStatus(s) == s },
	"isPaid":        func(p bool) bool { return p },
	"deref":         derefInt64,
	"deref0":        derefInt64Str,
	"firstNonEmpty": firstNonEmptyStr,
	"containsStr":   containsString,
	"currencyOf":    currencyOf,
	"subtract":      subtractFloat,
	"fmtDate":       fmtDate,
	"roleLabel":     roleLabel,
	"currencySym":   currencySym,
	"hasPrefix":     strings.HasPrefix,
	"lower":         strings.ToLower,
	"joinPath":      joinNonEmpty,
	"dict":          dict,
	"now":           func() time.Time { return time.Now() },
}

// fmtIDR renders full rupiah: 366000 -> "Rp 366.000".
func fmtIDR(v float64) string {
	// Use dot as thousand separator (Indonesian convention).
	n := int64(math.Round(v))
	neg := n < 0
	if neg {
		n = -n
	}
	s := formatThousands(n)
	if neg {
		return "Rp -" + s
	}
	return "Rp " + s
}

// fmtIDRShort renders compact: 366000 -> "366rb", 1500000 -> "1,5jt".
func fmtIDRShort(v float64) string {
	amt := int64(math.Round(v))
	switch {
	case amt >= 1_000_000:
		return floatToString(float64(amt)/1_000_000, 1) + "jt"
	case amt >= 1_000:
		return floatToString(float64(amt)/1_000, 0) + "rb"
	default:
		return formatThousands(amt)
	}
}

// floatToString formats with fixed precision.
func floatToString(v float64, prec int) string {
	mul := math.Pow(10, float64(prec))
	rounded := math.Round(v*mul) / mul
	whole := int64(rounded)
	frac := int64(math.Round((rounded - float64(whole)) * mul))
	sign := ""
	if whole < 0 || (whole == 0 && frac < 0) {
		sign = "-"
		whole = -whole
		frac = -frac
	}
	if prec == 0 {
		return sign + formatThousands(whole)
	}
	fracStr := ""
	f := frac
	for i := 0; i < prec; i++ {
		fracStr = string(rune('0'+int(f%10))) + fracStr
		f /= 10
	}
	return sign + formatThousands(whole) + "," + fracStr
}

// formatThousands inserts dots every 3 digits.
func formatThousands(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []rune{}
	for n > 0 {
		digits = append([]rune{rune('0' + int(n%10))}, digits...)
		n /= 10
	}
	var out []rune
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, d)
	}
	s := string(out)
	if neg {
		return "-" + s
	}
	return s
}

// fmtMoney is an alias for full IDR formatting.
func fmtMoney(v float64) string { return fmtIDR(v) }

// fmtForeign renders a foreign amount with its currency code.
func fmtForeign(amount float64, code string) string {
	if amount == 0 {
		return ""
	}
	return formatThousands(int64(math.Round(amount))) + " " + strings.ToUpper(code)
}

// fmt2 renders a 2-decimal number (for markup %).
func fmt2(v float64) string {
	return floatToString(v, 2)
}

// roleLabel gives the Bahasa label for a role.
func roleLabel(r auth.Role) string {
	switch r {
	case auth.RoleAdmin:
		return "Admin"
	case auth.RoleJastiper:
		return "Jastiper"
	default:
		return "Buyer"
	}
}

// currencySym returns a friendly symbol for common currencies.
func currencySym(code string) string {
	switch strings.ToUpper(code) {
	case "JPY":
		return "¥"
	case "KRW":
		return "₩"
	case "USD":
		return "$"
	case "SGD":
		return "S$"
	case "HKD":
		return "HK$"
	case "THB":
		return "฿"
	case "CNY":
		return "¥"
	case "TWD":
		return "NT$"
	case "MYR":
		return "RM"
	case "IDR":
		return "Rp"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "AUD":
		return "A$"
	case "PHP":
		return "₱"
	case "INR":
		return "₹"
	case "CAD":
		return "C$"
	case "NZD":
		return "NZ$"
	case "CHF":
		return "CHF "
	case "VND":
		return "₫"
	}
	return strings.ToUpper(code) + " "
}

func joinNonEmpty(parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

// dict creates a map from key/value pairs, for passing scoped vars in templates.
func dict(values ...any) map[string]any {
	m := map[string]any{}
	for i := 0; i+1 < len(values); i += 2 {
		k, ok := values[i].(string)
		if !ok {
			continue
		}
		m[k] = values[i+1]
	}
	return m
}

// derefInt64 returns the pointed-to value as int64, or 0 if nil.
func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// derefInt64Str returns the pointed-to value as a string ("0" if nil),
// for use in template `eq` comparisons against form values (which are strings).
func derefInt64Str(p *int64) string {
	if p == nil {
		return "0"
	}
	return formatInt64(*p)
}

// firstNonEmptyStr returns the first non-empty string arg, or "".
func firstNonEmptyStr(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// subtractFloat returns a - b.
func subtractFloat(a, b float64) float64 { return a - b }

// containsString reports whether the slice contains s. Used by the
// currency dropdown to decide whether to append a custom "(dari scan)" option.
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// currencyOf safely reads the Currency field from an order/scan-result value
// passed from a template, returning "" for nil or unknown types. The order
// form uses both orders.Order and scan.Result, and either may be nil when
// creating a brand-new order, so direct field access panics in templates.
func currencyOf(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case orders.Order:
		return x.Currency
	}
	// scan.Result lives in another package; reach it via reflection to avoid
	// an import cycle (web -> scan is fine, but we keep this generic).
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Struct {
		f := rv.FieldByName("Currency")
		if f.IsValid() && f.Kind() == reflect.String {
			return f.String()
		}
	}
	return ""
}

// fmtDate renders a time.Time as "2 Jan 2006" (Indonesian-friendly).
func fmtDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	months := []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	return fmt.Sprintf("%d %s %d", t.Day(), months[int(t.Month())-1], t.Year())
}

// render writes a template by name with the given data.
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data map[string]any, status ...int) {
	if data == nil {
		data = map[string]any{}
	}
	if u, ok := auth.UserFrom(r); ok {
		data["currentUser"] = u
		// For jastiper/admin, expose the pending-request count so the bottom
		// nav can show a badge. Best-effort: ignore errors (renders as 0).
		if u.Role == auth.RoleJastiper || u.Role == auth.RoleAdmin {
			if n, err := s.requests.CountPending(r.Context(), u.ID); err == nil {
				data["pendingRequests"] = n
			}
		}
	}
	data["supportedCurrencies"] = currency.Supported
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(templateFS,
		"templates/layout.html", "templates/"+name, "templates/partials/*.html")
	if err != nil {
		log.Printf("template parse error (%s): %v", name, err)
		http.Error(w, "template parse error", http.StatusInternalServerError)
		return
	}
	code := http.StatusOK
	if len(status) > 0 {
		code = status[0]
	}
	w.WriteHeader(code)
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		// Best-effort; header already sent, but at least log so blank pages
		// (response already started, then failed mid-stream) are debuggable.
		log.Printf("template exec error (%s): %v", name, err)
		_ = err
	}
}

// renderPartial writes a single template fragment (for HTMX swaps).
// `name` is the file name under templates/partials/ (e.g. "order_card.html");
// the template's defined block name is derived by stripping the extension.
func (s *Server) renderPartial(w http.ResponseWriter, name string, data map[string]any, status ...int) {
	tmpl, err := template.New("").Funcs(templateFuncs).ParseFS(templateFS, "templates/partials/"+name)
	if err != nil {
		http.Error(w, "template parse error", http.StatusInternalServerError)
		return
	}
	block := strings.TrimSuffix(name, ".html")
	code := http.StatusOK
	if len(status) > 0 {
		code = status[0]
	}
	w.WriteHeader(code)
	if err := tmpl.ExecuteTemplate(w, block, data); err != nil {
		http.Error(w, "template exec error", http.StatusInternalServerError)
	}
}
