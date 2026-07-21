// TitipDong — small client-side helpers shared across pages.

// copyMessage fetches the per-order status-update message from the server
// and copies it to the clipboard. Used when the buyer has no WhatsApp and
// the jastiper wants to paste the message into Instagram DM, Telegram, etc.
window.copyMessage = function (orderID, btn) {
  const original = btn.textContent;
  fetch('/app/orders/' + orderID + '/message')
    .then(function (r) { return r.text(); })
    .then(function (msg) {
      // Clipboard API (HTTPS-only). Fallback to a hidden textarea for HTTP.
      if (navigator.clipboard && window.isSecureContext) {
        return navigator.clipboard.writeText(msg);
      }
      const ta = document.createElement('textarea');
      ta.value = msg;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    })
    .then(function () {
      btn.textContent = '✓ Tersalin!';
      btn.classList.add('bg-emerald-100', 'text-emerald-700');
      setTimeout(function () {
        btn.textContent = original;
        btn.classList.remove('bg-emerald-100', 'text-emerald-700');
      }, 2000);
    })
    .catch(function () {
      btn.textContent = '✗ Gagal, coba lagi';
      setTimeout(function () { btn.textContent = original; }, 2000);
    });
};
