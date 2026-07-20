package scan

import "testing"

func TestNormalizeCurrency(t *testing.T) {
	cases := map[string]string{
		// Indonesian Rupiah — local symbol variants
		"rp":      "IDR",
		"Rp":      "IDR",
		"RP":      "IDR",
		"rupiah":  "IDR",
		"Rupiah":  "IDR",
		// Malaysia
		"RM":       "MYR",
		"ringgit":  "MYR",
		// Japan
		"¥":   "JPY",
		"YEN": "JPY",
		"yen": "JPY",
		// China
		"RMB":  "CNY",
		"yuan": "CNY",
		// Korea
		"₩":   "KRW",
		"WON": "KRW",
		// Thailand
		"฿":    "THB",
		"baht": "THB",
		// US dollar (default for ambiguous $)
		"$":      "USD",
		"dollar": "USD",
		// Singapore / HK / Taiwan / Australia
		"S$":  "SGD",
		"HK$": "HKD",
		"NT$": "TWD",
		"A$":  "AUD",
		// Europe
		"€":     "EUR",
		"euro":  "EUR",
		"£":     "GBP",
		"pound": "GBP",
		// Philippines / India / Vietnam / Canada / NZ / Switzerland
		"₱":     "PHP",
		"peso":  "PHP",
		"₹":     "INR",
		"rupee": "INR",
		"₫":     "VND",
		"dong":  "VND",
		"C$":    "CAD",
		"NZ$":   "NZD",
		"CHF":   "CHF",
		"franc": "CHF",
		// Already ISO codes pass through
		"JPY": "JPY",
		"IDR": "IDR",
		"USD": "USD",
		// Unknown code passes through upper-cased
		"xyz": "XYZ",
		"":    "",
	}
	for in, want := range cases {
		if got := normalizeCurrency(in); got != want {
			t.Errorf("normalizeCurrency(%q) = %q, want %q", in, got, want)
		}
	}
}
