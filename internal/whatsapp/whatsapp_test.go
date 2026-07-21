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
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatIDR(t *testing.T) {
	cases := map[float64]string{
		366_000:   "Rp 366rb",
		1_500_000: "Rp 1,5jt",
		500:       "Rp 500",
	}
	for amt, want := range cases {
		if got := FormatIDR(amt); got != want {
			t.Errorf("FormatIDR(%v) = %q, want %q", amt, got, want)
		}
	}
}

func TestMessageStatuses(t *testing.T) {
	withMessage := []orders.Status{
		orders.StatusPendingConfirmation, orders.StatusAccepted, orders.StatusRejected,
		orders.StatusWaitingForPayment, orders.StatusPaid, orders.StatusDelivery,
	}
	for _, st := range withMessage {
		msg := Message("Bu Yuni", "Hada Labo", st, 238000)
		if msg == "" {
			t.Errorf("Message for %v should not be empty", st)
		}
		if !strings.Contains(msg, "Yuni") || !strings.Contains(msg, "Hada") {
			t.Errorf("Message for %v should contain name+item: %q", st, msg)
		}
	}
	noMessage := []orders.Status{orders.StatusFinished, orders.StatusBuyerCancelled, orders.StatusSellerCancelled}
	for _, st := range noMessage {
		msg := Message("Yuni", "Item", st, 100)
		if msg != "" {
			t.Errorf("Message for %v should be empty, got %q", st, msg)
		}
	}
}

func TestComposeLink(t *testing.T) {
	link := ComposeLink("081234567890", "Bu Yuni", "Hada Labo", orders.StatusAccepted, 238000)
	if !strings.HasPrefix(link, "https://wa.me/6281234567890?text=") {
		t.Fatalf("link prefix wrong: %s", link)
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
