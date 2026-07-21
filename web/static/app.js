// TitipDong — small client-side helpers shared across pages.

// orderForm is the Alpine.js component state for the new/edit order form.
// It holds amount, fx rate, markup, currency, and a derived `selling` value.
// The rate is pre-filled (from the order snapshot when editing, or fetched
// live on new orders) but editable: jastiper can type their own rate from
// a money changer when the cached 24h rate is stale.
window.orderForm = function (initial) {
  return {
    amount: initial.amount || '',
    rate: initial.rate || '0',
    markup: initial.markup || '20',
    currency: initial.currency || 'JPY',
    selling: 0,

    init() {
      // Set up reactive watchers so the preview updates on every keystroke.
      this.$watch('amount', () => this.calc());
      this.$watch('rate', () => this.calc());
      this.$watch('markup', () => this.calc());
      this.$watch('currency', () => this.refreshRate());
      // Fetch rate on new orders (snapshot is 0) and always recalc once.
      if (!this.rate || this.rate === '0') {
        this.refreshRate();
      } else {
        this.calc();
      }
    },

    calc() {
      const a = parseFloat(String(this.amount).replace(/[^\d.-]/g, '')) || 0;
      const r = parseFloat(String(this.rate).replace(/[^\d.-]/g, '')) || 0;
      const m = parseFloat(this.markup) || 0;
      this.selling = a * r * (1 + m / 100);
    },

    async refreshRate() {
      try {
        const r = await fetch('/app/orders/fx?currency=' + encodeURIComponent(this.currency));
        const t = (await r.text()).trim();
        const parsed = parseFloat(t);
        if (parsed > 0) {
          this.rate = String(parsed);
          this.calc();
        }
      } catch (e) {
        // network error - leave current rate alone
      }
    },
  };
};

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
