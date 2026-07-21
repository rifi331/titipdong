# Change Log — TitipDong

Format tiap entry mengikuti template standar (Author / Date / Changes / DB / Detail).

---

Version v0.6.6 - fix photo upload permission (distroless → alpine)
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - fix photo upload (catalog/scan/KTP) yang silently gagal
D. DB: N/A
E. Detail:
    - Root cause: distroless image run sebagai user `nonroot`, tapi `/app`
      owner-nya root. `MkdirAll('/app/uploads')` permission denied.
    - Handler saveUpload error di-swallow diam-diam (`if err == nil`),
      jadi keliatan sukses padahal photo_path kosong di DB.
    - Switch distroless → alpine + `mkdir /app/uploads && chown nonroot`.
* Rest endpoint: N/A
* SQL script: N/A
* Go: N/A
* Dockerfile
    - (modified) Dockerfile — ganti FROM distroless ke alpine, add mkdir/chown
* Property: N/A

Version v0.6.5 - expand currency normalization
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - normalizeCurrency map ~60 local forms (Rp, RM, ¥, $, dll) ke ISO code
    - expand Supported list (+PHP, INR, CAD, NZD, CHF)
D. DB: N/A
E. Detail:
    - Scan struk sering return simbol lokal (Rp, RM, ¥, $) bukan ISO (IDR, MYR, JPY, USD).
    - Tanpa map, dropdown currency di form order gak match → keliatan kosong.
* Rest endpoint: N/A
* SQL script: N/A
* Go
    - (modified) internal/scan/scan.go — normalizeCurrency expanded
    - (modified) internal/currency/currency.go — Supported list +5
    - (modified) internal/web/render.go — currencySym +6
    - (new) internal/scan/scan_test.go — 40+ test cases
* Property: N/A

Version v0.6.4 - scan currency map + FX domain
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - add normalizeCurrency (RM→MYR, ¥→JPY, $→USD, dll)
    - FX_BASE_URL default: frankfurter.app (deprecated) → frankfurter.dev/v1
    - order_form currency dropdown: append "(dari scan)" option kalau value gak match
D. DB: N/A
E. Detail:
    - Bug: scan struk Malaysia return "RM", dropdown currency kosong
    - Bug: frankfurter.app 301 deprecated, MYR rate gak ter-fetch
* Rest endpoint: N/A
* SQL script: N/A
* Go
    - (modified) internal/scan/scan.go — normalizeCurrency baru
    - (modified) internal/config/config.go — FX_BASE_URL default
    - (modified) internal/web/render.go — containsStr helper
    - (modified) internal/web/templates/order_form.html — currency fallback
* Property: FX_BASE_URL default berubah jadi https://api.frankfurter.dev/v1

Version v0.6.3 - fix scan 500 (gob) + panic logger
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - fix scan struk HTTP 500 ("gob: type not registered for interface: scan.Result")
    - ganti Recoverer silent → logRecoverer yang log panic + stack trace
D. DB: N/A
E. Detail:
    - scs (session manager) serialize session via encoding/gob.
    - Saat scan handler simpan scan.Result ke session → gob gak kenal type → panic.
    - Recoverer lama balas 500 diam-diam tanpa log apapun.
* Rest endpoint: N/A
* SQL script: N/A
* Go
    - (modified) cmd/titipdong/main.go — gob.Register(scan.Result{})
    - (modified) internal/web/server.go — logRecoverer middleware
* Property: N/A

Version v0.6.2 - fix scan 500 (attempt 1, broken build)
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - (gob.Register — pertama ditambah, tapi duplicate config.Load bikin build gagal)
D. DB: N/A
E. Detail: BROKEN — lihat v0.6.3 untuk fix yang bener
* Status: do not use, superseded by v0.6.3

Version v0.6.1 - log panic + stack trace
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - ganti chi Recoverer (silent) → logRecoverer (log panic + stack)
D. DB: N/A
E. Detail:
    - Recoverer default balas 500 tanpa log apapun — bikin production bug
      impossible to debug dari docker logs.
* Go
    - (modified) internal/web/server.go
* Property: N/A

Version v0.6.0 - anonymous buyer request + custom request + fee model
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - buyer anonymous request via catalog ("Mau Ini!") — no login required
    - buyer custom request (barang di luar katalog) → /request
    - jastiper dashboard request queue dengan badge count
    - accept request dengan 2 fee model: percent harga atau per-kg
    - WA notif ke jastiper (saat buyer submit) dan ke buyer (saat accept/reject)
D. DB:
    - migration 0002_buyer_requests: tabel buyer_requests + enum request_status
    - migration 0003_custom_requests: extend buyer_requests (item snapshot columns,
      fee_model enum, fee_percent, fee_per_kg_idr, item_origin, item_est_weight_kg)
E. Detail:
    - Tabel buyer_requests support 2 tipe: catalog (catalog_item_id set) atau
      custom (catalog_item_id NULL, buyer isi sendiri).
    - Accept handler transactional: create customer + order + mark accepted + set fee.
    - Fee percent: selling = (harga × fx) × (1 + fee_percent/100)
    - Fee per_kg: selling = berat_kg × fee_per_kg_idr
* Rest endpoint
    - (new) GET  /catalog/{id}/request — public form (catalog item)
    - (new) POST /catalog/{id}/request — submit catalog request
    - (new) GET  /request — public form (custom item)
    - (new) POST /request — submit custom request
    - (new) GET  /catalog/thanks — landing page post-submit
    - (new) GET  /app/requests — jastiper dashboard
    - (new) POST /app/requests/{id}/accept — convert ke order+customer
    - (new) POST /app/requests/{id}/reject — mark rejected
    - (new) GET  /app/requests/{id}/wa — WA link ke buyer
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
    - (modified) internal/web/server.go — store + routes
    - (modified) internal/web/render.go — pendingRequests count
    - (modified) internal/kyc/kyc.go — PhoneForUser
    - (modified) internal/whatsapp/whatsapp.go — buyer request messages
    - (modified) internal/catalog/catalog.go — GetPublic + ErrNotFound
    - (modified) internal/web/templates/layout.html — bottom nav + badge
    - (new) internal/web/templates/custom_request_form.html
    - (modified) internal/web/templates/catalog_public.html — Mau Ini + link /request
    - (new) internal/web/templates/request_form.html
    - (new) internal/web/templates/request_thanks.html
    - (new) internal/web/templates/requests_dashboard.html
    - (new) internal/web/templates/partials/request_card.html
* Property: N/A

Version v0.5.0 - app version logging
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - log app version di startup via ldflags
D. DB: N/A
E. Detail:
    - Supaya user bisa verify image yang running via `docker logs titipdong`.
    - Solve issue "docker restart gak narik image baru" yang silent failure.
* Go
    - (new) internal/version/version.go
    - (modified) cmd/titipdong/main.go
* Dockerfile
    - (modified) APP_VERSION build-arg + ldflags
* Property: N/A
* Workflow
    - (modified) .github/workflows/publish.yml — pass github.ref_name ke APP_VERSION

Version v0.4.0 - photo picker (Xiaomi/Android camera fix)
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - fix tombol "Choose File" di Xiaomi 14T pro selalu buka kamera
    - 2 tombol: "Foto Langsung" (capture=environment) + "Pilih dari Galeri" (no capture)
D. DB: N/A
E. Detail:
    - Atribut `capture="environment"` di <input type=file> bikin Android selalu
      buka kamera, padahal user mau pilih foto dari galeri.
    - JS toggle capture attribute sebelum trigger .click() di 1 input yang sama
      (hindarin bug FormFile saat 2 input dengan name="photo").
* Go
    - (new) web/static/photo-picker.js
    - (modified) internal/web/templates/layout.html — load photo-picker.js
    - (modified) internal/web/templates/scan.html — 2 tombol picker
    - (modified) internal/web/templates/order_form.html — foto item
    - (modified) internal/web/templates/catalog.html — foto catalog
    - (modified) internal/web/templates/profile.html — foto KTP
* Property: N/A

Version v0.3.0 - regression check + gitignore testing docs
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - hapus SKENARIO_TESTING.md dan HASIL_TESTING_*.md dari repo publik + history
    - gitignore testing docs (internal QA, bukan publik)
    - hapus LAN IP dari docker-compose.truenas.yml + history git
D. DB: N/A
E. Detail:
    - file testing internal (skenario + hasil) gitignored, tetap di lokal
* Go: N/A
* Property: N/A

Version v0.2.1 - fix .gitignore exclude cmd/titipdong
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - fix .gitignore pattern `titipdong` ke `/titipdong` (anchor ke root)
D. DB: N/A
E. Detail:
    - Pattern tanpa `/` match semua folder bernama `titipdong`, termasuk
      `cmd/titipdong/` (entry point). CI gak bisa build.
* Go
    - (new) cmd/titipdong/main.go (yang sempat ke-ignore)

Version v0.2.0 - fix Go image match go.mod
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-21
C. Changes:
    - Dockerfile: golang:1.22-alpine → golang:1.25-alpine
D. DB: N/A
E. Detail:
    - go.mod declare go 1.25.x, image 1.22 too old → `go mod download` gagal
* Dockerfile: (modified)

Version v0.1.0 - initial release
----------------------------------------------------------------------------------------------
A. Author: ZCode
B. Date: 2026-07-20
C. Changes:
    - initial implementation TitipDong (jastip business tracker)
D. DB:
    - migration 0001_init: users, jastiper_applications, customers, trips,
      orders, catalog_items, fx_rates + enum types
E. Detail:
    - Mobile-first PWA untuk jastip merchant Indonesia.
    - Single Go binary (chi + pgx + scs + HTMX/Alpine/Tailwind).
    - Roles: Buyer → (KYC) → Jastiper → Admin.
    - Multi-currency dengan FX live (frankfurter), snapshot per order.
    - Order pipeline: Dicari → Ketemu → Dibeli → Dibayar → Diantar.
    - WA deep links untuk update customer.
    - Trip dashboard + payments + end-of-trip summary.
    - Receipt scan via OpenAI gpt-4o-mini.
    - Dockerfile multi-stage, docker-compose, GHCR workflow.
* SQL script
    - (new) internal/db/migrations/0001_init.up.sql / .down.sql
* Go
    - initial codebase (auth, catalog, currency, customers, db, kyc, orders,
      scan, trips, version, web, whatsapp)
* Property: semua env vars (DATABASE_URL, SESSION_SECRET, ADMIN_EMAIL, dst.)
