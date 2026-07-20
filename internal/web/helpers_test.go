package web

import "testing"

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"1800", 1800},
		{"1.800", 1800},       // ID thousands
		{"1,800.50", 1800.50}, // US
		{"1.800,50", 1800.50}, // EU
		{"1800,50", 1800.50},
		{"¥1800", 1800},
		{"", 0},
		{"Rp 366.000", 366000},
	}
	for _, c := range cases {
		if got := parseAmount(c.in); got != c.want {
			t.Errorf("parseAmount(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseOptionalInt(t *testing.T) {
	if parseOptionalInt("") != nil {
		t.Error("empty should be nil")
	}
	if parseOptionalInt("0") != nil {
		t.Error("0 should be nil")
	}
	p := parseOptionalInt("42")
	if p == nil || *p != 42 {
		t.Errorf("42 should resolve, got %v", p)
	}
}
