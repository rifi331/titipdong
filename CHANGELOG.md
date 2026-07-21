# Change Log — TitipDong

Each entry follows the standard template (Author / Date / Changes / DB / Detail).

---

Version v0.7.0 - admin FX rate refresh + editable rate in order form
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - FX rate in the order form is pre-filled from DB (or Frankfurter on miss)
      AND editable by the jastiper
    - new admin page "Kurs Valuta" with a "Refresh Rate Terbaru" button that
      pulls fresh rates for every supported currency from Frankfurter and
      stores them in the DB
D. DB: N/A (reuses existing fx_rates table)
E. Detail:
    - Rate() priority: (1) DB fresh (<24h), (2) Frankfurter live + cache,
      (3) DB stale as last resort, (4) zero on total failure.
    - Admin clicks "Refresh" -> currency.RefreshAll() fetches all supported
      currencies and overwrites the DB rows. Admin then sees the result of
      every currency (ok with rate, or error message).
    - The order form's "Kurs" input is pre-filled from the same Rate() call
      and the jastiper can override the value directly (e.g. their own money
      changer rate). Server stores the override as fx_rate_snapshot.
    - Admin home now has a "💱 Kurs Valuta" tile linking to the new page.
* Rest endpoint
    - (new) GET  /app/admin/rates - list cached rates + age + stale flag
    - (new) POST /app/admin/rates/refresh - fetch fresh rates from Frankfurter
    - (new) GET  /app/orders/fx?currency=XXX - plain-text rate (added v0.6.8)
    - (new) GET  /app/orders/{id}/message - plain-text status msg (added v0.6.8)
* SQL script: N/A
* Go
    - (modified) internal/currency/currency.go - Rate() priority doc + RefreshAll + SupportedCodes
    - (new) internal/web/handlers_rates.go - admin rates + refresh handlers
    - (modified) internal/web/handlers_orders.go - parse fx_rate_override, fall back to Rate()
    - (modified) internal/web/server.go - register /app/admin/rates* routes
    - (modified) internal/web/templates/order_form.html - new "Kurs" input,
      consolidated Alpine.js component (orderForm in app.js)
    - (modified) internal/web/templates/home_admin.html - Kurs Valuta tile
    - (new) internal/web/templates/admin_rates.html
    - (modified) web/static/app.js - orderForm Alpine component
    - (modified) internal/web/templates/layout.html - [x-cloak] style
* Property: N/A

Version v0.6.9 - editable FX rate in order form
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - FX rate in the order form is now pre-filled AND editable
D. DB: N/A
E. Detail:
    - Previously the rate was hidden and computed server-side only. Now the
      jastiper sees the rate (1 foreign unit = N IDR) and can edit it when the
      24h-cached rate is stale (e.g. they have a fresher rate from their own
      money changer).
    - On new orders: rate is fetched live from /app/orders/fx and pre-filled.
    - On edit: rate is pre-filled from the stored fx_rate_snapshot.
    - Changing the currency dropdown re-fetches the rate automatically.
    - Selling-price preview (added in v0.6.8) updates from whatever rate the
      jastiper types.
* Rest endpoint: N/A
* SQL script: N/A
* Go
    - (modified) internal/web/handlers_orders.go - parse fx_rate_override from
      form; fall back to FX service only when blank/0
    - (modified) internal/web/templates/order_form.html - new "Kurs" input,
      consolidated Alpine.js component
    - (modified) web/static/app.js - new orderForm() Alpine component with
      amount/rate/markup state, calc(), refreshRate()
    - (modified) internal/web/templates/layout.html - [x-cloak] style
* Property: N/A

Version v0.6.8 - live selling price preview + copy-message button
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - show estimated selling price live in the order form (updates as the
      jastiper types amount / markup / changes currency)
    - add "Copy Pesan" button to order cards as an alternative to WhatsApp
      (for buyers on Instagram DM, Telegram, SMS, etc.)
D. DB: N/A
E. Detail:
    - Live preview uses Alpine.js + a new GET /app/orders/fx?currency=XXX
      endpoint that returns the cached IDR rate as plain text. The form
      re-fetches the rate whenever the currency dropdown changes, then
      computes selling = amount x rate x (1 + markup/100) client-side.
    - "Copy Pesan" fetches the per-order status message from
      GET /app/orders/{id}/message and copies it to the clipboard via the
      Clipboard API (with a textarea fallback for HTTP). The jastiper can
      then paste it into any chat app.
* Rest endpoint
    - (new) GET /app/orders/fx?currency=XXX - plain-text IDR rate
    - (new) GET /app/orders/{id}/message - plain-text status message
* SQL script: N/A
* Go
    - (new) internal/web/handlers_fx.go
    - (new) internal/web/handlers_message.go
    - (modified) internal/web/server.go - register the two new routes
    - (modified) internal/web/templates/order_form.html - Alpine.js live preview
    - (modified) internal/web/templates/partials/order_card.html - copy button
    - (modified) internal/web/templates/layout.html - load app.js
    - (new) web/static/app.js - copyMessage helper
* Property: N/A

Version v0.6.7 - fix order_form template panic (nil-safe currency access)
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - fix scan-receipt flow that only rendered 2 fields then stopped
D. DB: N/A
E. Detail:
    - Symptom: after scanning a receipt, the redirect to /app/orders/new
      only showed the item + store fields, then the page cut off.
    - Server log: `template exec error (order_form.html): ... at
      <$o.Currency>: invalid value; expected string`.
    - Root cause: when creating a new order, $o (orders.Order) is nil. The
      template accessed $o.Currency directly, which panics on nil. The
      logRecoverer middleware (added in v0.6.1) caught the panic but the
      page render was already truncated.
    - Fix: new currencyOf(v any) string helper uses reflection to safely
      read the Currency field from orders.Order or scan.Result (or returns
      "" for nil). Template now uses {{currencyOf $o}} {{currencyOf $sr}}.
* Rest endpoint: N/A
* SQL script: N/A
* Go
    - (modified) internal/web/render.go - currencyOf helper + reflect import
    - (modified) internal/web/templates/order_form.html - use currencyOf
* Property: N/A

Version v0.6.6 - fix photo upload permission (distroless -> alpine)
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - fix photo upload (catalog/scan/KTP) that silently failed
D. DB: N/A
E. Detail:
    - Root cause: the distroless image runs as the `nonroot` user, but `/app`
      is owned by root. `MkdirAll('/app/uploads')` returned permission denied.
    - The saveUpload handler swallowed the error silently (`if err == nil`),
      so the API looked successful but `photo_path` was empty in the DB.
    - Switched base image from distroless to alpine and added
      `mkdir /app/uploads && chown nonroot` so the runtime user can write.
* Rest endpoint: N/A
* SQL script: N/A
* Go: N/A
* Dockerfile
    - (modified) Dockerfile - replaced FROM distroless with alpine, added mkdir/chown
* Property: N/A

Version v0.6.5 - expand currency normalization
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - normalizeCurrency now maps ~60 local forms (Rp, RM, Yen, $, etc.) to ISO codes
    - expanded Supported list (+PHP, INR, CAD, NZD, CHF)
D. DB: N/A
E. Detail:
    - Receipt scans often return local symbols (Rp, RM, Yen, $) instead of ISO
      codes (IDR, MYR, JPY, USD). Without a mapping, the currency dropdown in
      the order form did not match any option and looked empty.
* Rest endpoint: N/A
* SQL script: N/A
* Go
    - (modified) internal/scan/scan.go - normalizeCurrency expanded
    - (modified) internal/currency/currency.go - Supported list +5
    - (modified) internal/web/render.go - currencySym +6
    - (new) internal/scan/scan_test.go - 40+ test cases
* Property: N/A

Version v0.6.4 - scan currency map + FX domain
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - add normalizeCurrency (RM->MYR, yen-sign->JPY, $->USD, etc.)
    - FX_BASE_URL default: frankfurter.app (deprecated) -> frankfurter.dev/v1
    - order_form currency dropdown: append "(dari scan)" option when value does not match
D. DB: N/A
E. Detail:
    - Bug: scanning a Malaysian receipt returned "RM" and the currency dropdown was empty.
    - Bug: frankfurter.app returns 301 (deprecated) and MYR rates never loaded.
* Rest endpoint: N/A
* SQL script: N/A
* Go
    - (modified) internal/scan/scan.go - new normalizeCurrency function
    - (modified) internal/config/config.go - FX_BASE_URL default
    - (modified) internal/web/render.go - containsStr helper
    - (modified) internal/web/templates/order_form.html - currency fallback
* Property: FX_BASE_URL default changed to https://api.frankfurter.dev/v1

Version v0.6.3 - fix scan 500 (gob) + panic logger
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - fix scan receipt HTTP 500 ("gob: type not registered for interface: scan.Result")
    - replace silent chi Recoverer with logRecoverer that logs panic + stack trace
D. DB: N/A
E. Detail:
    - scs (session manager) serializes session data via encoding/gob.
    - When the scan handler stored scan.Result in the session, gob did not know
      the type and panicked.
    - The old Recoverer replied 500 silently without any log entry.
* Rest endpoint: N/A
* SQL script: N/A
* Go
    - (modified) cmd/titipdong/main.go - gob.Register(scan.Result{})
    - (modified) internal/web/server.go - logRecoverer middleware
* Property: N/A

Version v0.6.2 - fix scan 500 (attempt 1, broken build)
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - (gob.Register added here first, but a duplicate config.Load broke the build)
D. DB: N/A
E. Detail: BROKEN - see v0.6.3 for the correct fix
* Status: do not use, superseded by v0.6.3

Version v0.6.1 - log panic + stack trace
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - replace chi Recoverer (silent) with logRecoverer (logs panic + stack)
D. DB: N/A
E. Detail:
    - The default Recoverer replied 500 without any log, which made production
      bugs impossible to debug from docker logs.
* Go
    - (modified) internal/web/server.go
* Property: N/A

Version v0.6.0 - anonymous buyer request + custom request + fee model
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - anonymous buyer request via catalog ("Mau Ini!") - no login required
    - anonymous custom-item request (item not in catalog) -> /request
    - jastiper request dashboard with pending-count badge
    - accept request with two fee models: percent of price, or per kilogram
    - WA notification to jastiper (on submit) and to buyer (on accept/reject)
D. DB:
    - migration 0002_buyer_requests: buyer_requests table + request_status enum
    - migration 0003_custom_requests: extend buyer_requests (item snapshot columns,
      fee_model enum, fee_percent, fee_per_kg_idr, item_origin, item_est_weight_kg)
E. Detail:
    - The buyer_requests table supports two flavors: catalog (catalog_item_id set)
      and custom (catalog_item_id NULL, buyer fills item fields).
    - Accept handler is transactional: create customer + order, mark accepted, set fee.
    - Fee percent: selling = (price x fx) x (1 + fee_percent/100)
    - Fee per_kg: selling = weight_kg x fee_per_kg_idr
* Rest endpoint
    - (new) GET  /catalog/{id}/request - public form (catalog item)
    - (new) POST /catalog/{id}/request - submit catalog request
    - (new) GET  /request - public form (custom item)
    - (new) POST /request - submit custom request
    - (new) GET  /catalog/thanks - landing page after submit
    - (new) GET  /app/requests - jastiper dashboard
    - (new) POST /app/requests/{id}/accept - convert to order+customer
    - (new) POST /app/requests/{id}/reject - mark rejected
    - (new) GET  /app/requests/{id}/wa - WA link to buyer
* SQL script
    - (new) internal/db/migrations/0002_buyer_requests.up.sql
    - (new) internal/db/migrations/0002_buyer_requests.down.sql
    - (new) internal/db/migrations/0003_custom_requests.up.sql
    - (new) internal/db/migrations/0003_custom_requests.down.sql
* Go
    - (new) internal/requests/requests.go
    - (new) internal/web/handlers_requests.go
    - (new) internal/web/handlers_requests_admin.go
    - (new) internal/web/handlers_custom_request.go
    - (modified) internal/web/server.go - store + routes
    - (modified) internal/web/render.go - pendingRequests count
    - (modified) internal/kyc/kyc.go - PhoneForUser
    - (modified) internal/whatsapp/whatsapp.go - buyer request messages
    - (modified) internal/catalog/catalog.go - GetPublic + ErrNotFound
    - (modified) internal/web/templates/layout.html - bottom nav + badge
    - (new) internal/web/templates/custom_request_form.html
    - (modified) internal/web/templates/catalog_public.html - "Mau Ini" + link to /request
    - (new) internal/web/templates/request_form.html
    - (new) internal/web/templates/request_thanks.html
    - (new) internal/web/templates/requests_dashboard.html
    - (new) internal/web/templates/partials/request_card.html
* Property: N/A

Version v0.5.0 - app version logging
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - log app version on startup via ldflags
D. DB: N/A
E. Detail:
    - Lets the user verify which image is running via `docker logs titipdong`.
    - Solves the silent failure of "docker restart does not pull a new image".
* Go
    - (new) internal/version/version.go
    - (modified) cmd/titipdong/main.go
* Dockerfile
    - (modified) APP_VERSION build-arg + ldflags
* Property: N/A
* Workflow
    - (modified) .github/workflows/publish.yml - pass github.ref_name as APP_VERSION

Version v0.4.0 - photo picker (Xiaomi/Android camera fix)
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - fix "Choose File" button on Xiaomi 14T pro that always opened the camera
    - two buttons: "Foto Langsung" (capture=environment) + "Pilih dari Galeri" (no capture)
D. DB: N/A
E. Detail:
    - The `capture="environment"` attribute on <input type=file> made Android
      always open the camera, even when the user wanted to pick a photo from
      the gallery.
    - JS toggles the capture attribute right before .click() on the same input
      (avoids the FormFile bug that occurs with two inputs sharing name="photo").
* Go
    - (new) web/static/photo-picker.js
    - (modified) internal/web/templates/layout.html - load photo-picker.js
    - (modified) internal/web/templates/scan.html - two-button picker
    - (modified) internal/web/templates/order_form.html - item photo
    - (modified) internal/web/templates/catalog.html - catalog photo
    - (modified) internal/web/templates/profile.html - KTP photo
* Property: N/A

Version v0.3.0 - regression check + gitignore testing docs
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - remove SKENARIO_TESTING.md and HASIL_TESTING_*.md from public repo + history
    - gitignore testing docs (internal QA, not public)
    - remove LAN IP from docker-compose.truenas.yml + git history
D. DB: N/A
E. Detail:
    - Internal testing files (scenario + results) are gitignored; they remain local.
* Go: N/A
* Property: N/A

Version v0.2.1 - fix .gitignore exclude cmd/titipdong
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - fix .gitignore pattern `titipdong` to `/titipdong` (anchor to repo root)
D. DB: N/A
E. Detail:
    - A pattern without a leading `/` matches any folder named `titipdong`,
      including `cmd/titipdong/` (the entry point). CI could not build.
* Go
    - (new) cmd/titipdong/main.go (the file that had been ignored)

Version v0.2.0 - fix Go image match go.mod
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-21
C. Changes:
    - Dockerfile: golang:1.22-alpine -> golang:1.25-alpine
D. DB: N/A
E. Detail:
    - go.mod declares go 1.25.x; image 1.22 was too old, so `go mod download` failed.
* Dockerfile: (modified)

Version v0.1.0 - initial release
----------------------------------------------------------------------------------------------
A. Author: Rifi
B. Date: 2026-07-20
C. Changes:
    - initial implementation of TitipDong (jastip business tracker)
D. DB:
    - migration 0001_init: users, jastiper_applications, customers, trips,
      orders, catalog_items, fx_rates + enum types
E. Detail:
    - Mobile-first PWA for Indonesian jastip merchants.
    - Single Go binary (chi + pgx + scs + HTMX/Alpine/Tailwind).
    - Roles: Buyer -> (KYC) -> Jastiper -> Admin.
    - Multi-currency with live FX (frankfurter), rate snapshotted per order.
    - Order pipeline: Dicari -> Ketemu -> Dibeli -> Dibayar -> Diantar.
    - WA deep links for customer updates.
    - Trip dashboard + payments + end-of-trip summary.
    - Receipt scan via OpenAI gpt-4o-mini.
    - Multi-stage Dockerfile, docker-compose, GHCR workflow.
* SQL script
    - (new) internal/db/migrations/0001_init.up.sql / .down.sql
* Go
    - initial codebase (auth, catalog, currency, customers, db, kyc, orders,
      scan, trips, version, web, whatsapp)
* Property: all env vars (DATABASE_URL, SESSION_SECRET, ADMIN_EMAIL, etc.)
