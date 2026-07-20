package whatsapp

import (
	"strings"
	"testing"

	"github.com/titipdong/titipdong/internal/orders"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"+62 812-3456-7890": "6281234567890",
		"081234567890":      "6281234567890",
		"62 812 3456 7890":  "6281234567890",
		"abc":               "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatIDRShort(t *testing.T) {
	cases := map[float64]string{
		366_000:   "366rb",
		1_500_000: "1,5jt",
		500:       "500",
		999:       "999",
	}
	for amt, want := range cases {
		if got := FormatIDR(amt); got != "Rp "+want {
			t.Errorf("FormatIDR(%v) short = %q, want %q", amt, got, "Rp "+want)
		}
	}
}

func TestComposeLink(t *testing.T) {
	link := ComposeLink("081234567890", "Bu Yuni", "Hada Labo", orders.StatusKetemu, 366000)
	if !strings.HasPrefix(link, "https://wa.me/6281234567890?text=") {
		t.Fatalf("link prefix wrong: %s", link)
	}
	if !strings.Contains(link, "Hada") || !strings.Contains(link, "ketemu") {
		t.Errorf("message not encoded in link: %s", link)
	}
}

func TestMessage_Ketemu(t *testing.T) {
	msg := Message("Bu Yuni", "Hada Labo", orders.StatusKetemu, 366000)
	if !strings.Contains(msg, "Bu Yuni") || !strings.Contains(msg, "Hada Labo") || !strings.Contains(msg, "ketemu") {
		t.Errorf("ketemu message missing parts: %q", msg)
	}
}

func TestGreetingFor(t *testing.T) {
	cases := map[string]string{
		"":         "Kak",
		"Yuni":     "Yuni",
		"Bu Yuni":  "Bu Yuni",
		"Pak Budi": "Pak Budi",
	}
	for in, want := range cases {
		if got := greetingFor(in); got != want {
			t.Errorf("greetingFor(%q) = %q, want %q", in, got, want)
		}
	}
}
