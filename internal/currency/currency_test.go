package currency

import (
	"math"
	"testing"
)

func TestToIDR(t *testing.T) {
	if got := ToIDR(1800, 110); !floatEq(got, 198000) {
		t.Errorf("ToIDR(1800,110) = %v, want 198000", got)
	}
	if got := ToIDR(100, 0); got != 0 {
		t.Errorf("ToIDR with zero rate should be 0, got %v", got)
	}
}

func TestSellingPrice(t *testing.T) {
	// 1800 JPY @ 110 IDR * (1 + 20%) = 237600
	if got := SellingPrice(1800, 110, 20); !floatEq(got, 237600) {
		t.Errorf("SellingPrice(1800,110,20) = %v, want 237600", got)
	}
}

func floatEq(a, b float64) bool { return math.Abs(a-b) < 0.01 }
