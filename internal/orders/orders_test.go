package orders

import "testing"

func TestStatusLabel(t *testing.T) {
	cases := map[Status]string{
		StatusPendingConfirmation: "Menunggu Konfirmasi",
		StatusAccepted:            "Diterima",
		StatusWaitingForPayment:   "Menunggu Pembayaran",
		StatusPaid:                "Lunas",
		StatusDelivery:            "Diantar",
		StatusFinished:            "Selesai",
	}
	for st, want := range cases {
		if got := StatusLabel(st); got != want {
			t.Errorf("StatusLabel(%v) = %q, want %q", st, got, want)
		}
	}
}

func TestIsPaid(t *testing.T) {
	paid := []Status{StatusPaid, StatusDelivery, StatusFinished}
	for _, s := range paid {
		if !IsPaid(s) {
			t.Errorf("IsPaid(%v) should be true", s)
		}
	}
	notPaid := []Status{StatusPendingConfirmation, StatusAccepted, StatusWaitingForPayment,
		StatusRejected, StatusBuyerCancelled, StatusSellerCancelled}
	for _, s := range notPaid {
		if IsPaid(s) {
			t.Errorf("IsPaid(%v) should be false", s)
		}
	}
}
