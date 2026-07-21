// Package whatsapp builds wa.me deep links with pre-filled customer messages.
package whatsapp

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/titipdong/titipdong/internal/orders"
)

// Normalize strips non-digits from a phone field (keeps leading + as digits).
// e.g. "+62 812-3456-7890" -> "6281234567890".
func Normalize(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	// Convert leading 0 (local format) to 62 (Indonesia).
	if strings.HasPrefix(s, "0") {
		s = "62" + s[1:]
	}
	return s
}

// FormatIDR renders a rupiah amount compactly, e.g. 366000 -> "Rp 366rb", 1500000 -> "Rp 1,5jt".
func FormatIDR(amount float64) string {
	amt := int64(amount)
	switch {
	case amt >= 1_000_000:
		return "Rp " + commaDecimal(float64(amt)/1_000_000) + "jt"
	case amt >= 1_000:
		return fmt.Sprintf("Rp %drb", amt/1_000)
	default:
		return fmt.Sprintf("Rp %d", amt)
	}
}

// commaDecimal formats a value with one decimal place using a comma separator,
// trimming the trailing ",0" (so 1.0 -> "1", 1.5 -> "1,5").
func commaDecimal(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	s = strings.TrimSuffix(s, ".0")
	return strings.ReplaceAll(s, ".", ",")
}

// ComposeLink builds a wa.me URL with a pre-filled message for a status transition.
// customerName and item are the human-readable pieces; priceIDR may be 0 to omit.
func ComposeLink(phone, customerName, item string, st orders.Status, priceIDR float64) string {
	msg := Message(customerName, item, st, priceIDR)
	num := Normalize(phone)
	return fmt.Sprintf("https://wa.me/%s?text=%s", num, url.QueryEscape(msg))
}

// Message formats the friendly status-update text for a given lifecycle status.
// Returns "" for statuses that have no customer message (finished, cancelled).
func Message(customerName, item string, st orders.Status, priceIDR float64) string {
	greeting := greetingFor(customerName)
	price := ""
	if priceIDR > 0 {
		price = FormatIDR(priceIDR)
	}
	switch st {
	case orders.StatusPendingConfirmation:
		return fmt.Sprintf("Halo %s, request %s kami terima. Tunggu konfirmasi ya 🙏", greeting, item)
	case orders.StatusAccepted:
		return fmt.Sprintf("Halo %s, request %s udah aku terima! Aku kabarin pas udah siap ✨", greeting, item)
	case orders.StatusRejected:
		return fmt.Sprintf("Halo %s, maaf ya, request %s lagi gak bisa aku bantu 🙏", greeting, item)
	case orders.StatusWaitingForPayment:
		if price != "" {
			return fmt.Sprintf("Halo %s, %s siap! Total %s. Bayar ya sebelum aku kirim 💳", greeting, item, price)
		}
		return fmt.Sprintf("Halo %s, %s siap! Tunggu kabar totalnya ya 💳", greeting, item)
	case orders.StatusPaid:
		if price != "" {
			return fmt.Sprintf("Halo %s, pembayaran %s diterima%s. Makasih ya! 🙏", greeting, item, fmt.Sprintf(" %s", price))
		}
		return fmt.Sprintf("Halo %s, pembayaran %s diterima. Makasih ya! 🙏", greeting, item)
	case orders.StatusDelivery:
		return fmt.Sprintf("Halo %s, %s lagi diantar ya. Sampai ketemu! 📦", greeting, item)
	case orders.StatusFinished, orders.StatusBuyerCancelled, orders.StatusSellerCancelled:
		// No customer-facing message for terminal/cancelled statuses.
		return ""
	}
	return fmt.Sprintf("Halo %s, update %s: %s", greeting, item, orders.StatusLabel(st))
}

// greetingFor picks a friendly address from a name; falls back to "Kak".
func greetingFor(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Kak"
	}
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "bu ") || strings.HasPrefix(lower, "ibu ") {
		return "Bu " + strings.TrimSpace(name[strings.IndexByte(name, ' ')+1:])
	}
	if strings.HasPrefix(lower, "pak ") || strings.HasPrefix(lower, "mas ") || strings.HasPrefix(lower, "mbak ") {
		return name
	}
	return name
}

// TripSummaryMessage builds a friendly end-of-trip recap to share on WhatsApp.
func TripSummaryMessage(tripName string, orderCount int, revenueIDR, marginIDR float64, topCustomer string) string {
	top := topCustomer
	if top == "" {
		top = "-"
	}
	return fmt.Sprintf(
		"Trip %s selesai! 🎉\nTotal order: %d\nOmzet: %s\nMargin: %s\nTop customer: %s",
		tripName, orderCount, FormatIDR(revenueIDR), FormatIDR(marginIDR), top,
	)
}

// TripSummaryLink wraps TripSummaryMessage into a wa.me URL.
func TripSummaryLink(phone, tripName string, orderCount int, revenueIDR, marginIDR float64, topCustomer string) string {
	return fmt.Sprintf("https://wa.me/%s?text=%s",
		Normalize(phone), url.QueryEscape(TripSummaryMessage(tripName, orderCount, revenueIDR, marginIDR, topCustomer)))
}

// TripSummaryShareLink returns a wa.me/?text= link that opens the share screen
// without a specific recipient (used when no phone is known).
func TripSummaryShareLink(tripName string, orderCount int, revenueIDR, marginIDR float64, topCustomer string) string {
	return "https://wa.me/?text=" + url.QueryEscape(TripSummaryMessage(tripName, orderCount, revenueIDR, marginIDR, topCustomer))
}

// BuyerRequestToJastiperMessage is the message a buyer sends (via the public
// catalog) to the jastiper who owns an item. The buyer taps "Mau Ini!", submits
// name+WhatsApp+note, then opens WA with this pre-filled text.
func BuyerRequestToJastiperMessage(jastiperName, buyerName, itemTitle, note string) string {
	msg := fmt.Sprintf("Halo %s, aku %s tertarik sama %s di katalog TitipDong.",
		greetingFor(jastiperName), strings.TrimSpace(buyerName), itemTitle)
	if n := strings.TrimSpace(note); n != "" {
		msg += fmt.Sprintf("\n\nCatatan: %s", n)
	}
	msg += "\n\nBisa dibantu konfirmasi harga & ketersediaan? 🙏"
	return msg
}

// BuyerRequestToJastiperLink wraps the message into a wa.me URL addressed to the
// jastiper's phone.
func BuyerRequestToJastiperLink(jastiperPhone, jastiperName, buyerName, itemTitle, note string) string {
	return fmt.Sprintf("https://wa.me/%s?text=%s",
		Normalize(jastiperPhone),
		url.QueryEscape(BuyerRequestToJastiperMessage(jastiperName, buyerName, itemTitle, note)))
}

// JastiperToBuyerAcceptMessage is what the jastiper sends back to the buyer
// from their dashboard after accepting a request.
func JastiperToBuyerAcceptMessage(buyerName, itemTitle, estPriceForeign, currency string, estIDR float64) string {
	priceHint := ""
	if estIDR > 0 {
		priceHint = fmt.Sprintf(" Estimasi sekitar %s.", FormatIDR(estIDR))
	}
	return fmt.Sprintf(
		"Halo %s, request %s udah aku terima!%s Aku kabarin lagi pas barangnya udah aku beli ya ✨",
		greetingFor(buyerName), itemTitle, priceHint,
	)
}

// JastiperToBuyerAcceptLink wraps the accept message into a wa.me URL.
func JastiperToBuyerAcceptLink(buyerPhone, buyerName, itemTitle string, estPriceForeign float64, currency string, estIDR float64) string {
	return fmt.Sprintf("https://wa.me/%s?text=%s",
		Normalize(buyerPhone),
		url.QueryEscape(JastiperToBuyerAcceptMessage(buyerName, itemTitle, fmt.Sprintf("%.0f %s", estPriceForeign, currency), currency, estIDR)))
}

// JastiperToBuyerRejectMessage is what the jastiper sends when declining.
func JastiperToBuyerRejectMessage(buyerName, itemTitle, reason string) string {
	msg := fmt.Sprintf("Halo %s, maaf ya, request %s lagi gak bisa aku bantu", greetingFor(buyerName), itemTitle)
	if r := strings.TrimSpace(reason); r != "" {
		msg += fmt.Sprintf(" (%s)", r)
	}
	msg += ". Cek katalog aku lain kali ya! 🙏"
	return msg
}

// JastiperToBuyerRejectLink wraps the reject message into a wa.me URL.
func JastiperToBuyerRejectLink(buyerPhone, buyerName, itemTitle, reason string) string {
	return fmt.Sprintf("https://wa.me/%s?text=%s",
		Normalize(buyerPhone),
		url.QueryEscape(JastiperToBuyerRejectMessage(buyerName, itemTitle, reason)))
}
