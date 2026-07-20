# TitipDong 🛍️

**Tracker bisnis jastip untuk penjual titip-belanja-luar-negeri.**

Mobile-first PWA yang dipakai sambil jalan di toko luar negeri: catat order
multi-currency, otomatis konversi ke Rupiah, update customer lewat WhatsApp,
kelola trip, dan lihat margin — semua dari HP. Backend Go + PostgreSQL, satu
container, siap deploy ke TrueNAS lewat GHCR.

> *"Halo Bu Yuni, Hada Labo ketemu, Rp 238rb, konfirmasi ya?"*

## Fitur

- **Multi-currency** — input dalam JPY/KRW/SGD/THB/HKD/USD/dll, otomatis
  dikonversi ke IDR pakai exchange rate live (frankfurter.app, gratis tanpa
  API key). Rate di-snapshot per order saat disimpan.
- **Order pipeline** satu-tap: `Dicari → Ketemu → Dibeli → Dibayar → Diantar`.
- **Update WhatsApp otomatis** — setiap transisi status bikin link `wa.me`
  dengan pesan pre-filled (kamu tinggal tap Send).
- **Trip dashboard** — total order, omzet, modal, net margin, rincian per
  customer, top store.
- **Payment tracking** — tandai lunas, lihat piutang per customer.
- **Scan struk (opsional)** — foto struk toko luar negeri, OpenAI vision
  membaca item + toko + currency + harga, form order diisi otomatis.
- **End-of-trip summary** — ringkasan omzet/margin/top customer, shareable
  ke WhatsApp.
- **PWA** — bisa di-install ke home screen, ada service worker untuk offline.

## Role & KYC

Alur progresif: **Buyer → (ajukan KYC) → Jastiper → Admin**.

| Aksi | Buyer | Jastiper | Admin |
|---|---|---|---|
| Daftar, lihat katalog, ajukan jadi Jastiper | ✅ | ✅ | ✅ |
| Buat order, customer, trip, pipeline, WhatsApp, scan struk | ❌ | ✅ | ✅ |
| Dashboard, pembayaran, ringkasan trip | ❌ | ✅ | ✅ |
| Kelola user, approve/reject KYC, ubah role | ❌ | ❌ | ✅ |

Setiap signup baru jadi **Buyer**. Buyer yang mau jualan mengisi form KYC
(nama lengkap, nomor KTP, foto KTP, HP, kota). **Admin** me-review dan
approve → role naik jadi **Jastiper**.

## Tech stack

- **Go 1.22**, router `chi/v5`, DB `pgx/v5`, session `scs/v2`, password `bcrypt`.
- **PostgreSQL 16** (schema via embedded migrations, auto-run saat boot).
- **Frontend**: HTML server-rendered + HTMX + Alpine.js + Tailwind CSS
  (semua di-vendor, tidak butuh CDN di runtime).
- **OpenAI `gpt-4o-mini`** vision untuk scan struk (opsional).
- Satu binary + satu container.

## Konfigurasi (env)

Lihat [`.env.example`](./.env.example) untuk daftar lengkap. Yang wajib:

| Env | Keterangan |
|---|---|
| `DATABASE_URL` | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `SESSION_SECRET` | minimal 32 byte random (signing cookie) |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | admin pertama, dibuat saat boot jika belum ada |
| `BASE_URL` | URL publik deployment (untuk PWA manifest) |
| `OPENAI_API_KEY` | (opsional) kalau kosong, fitur scan struk disembunyikan |
| `FX_BASE_URL` | (opsional) sumber exchange rate, default `https://api.frankfurter.app` |
| `UPLOADS_DIR` | (opsional) lokasi simpan foto, default `./uploads` |

## Deploy ke TrueNAS

### Opsi A — pakai docker-compose (paling gampang)

TrueNAS Scale mendukung jalankan docker-compose lewat "Custom App" atau SSH.

1. **Buat image di GHCR.** Tag `v1.0.0` di repo GitHub memicu workflow
   `.github/workflows/publish.yml` yang build & push image multi-arch
   (`linux/amd64`, `linux/arm64`) ke
   `ghcr.io/<github-username>/titipdong:latest`.
   Ganti `GHCR_OWNER` di `.env` dengan username/org GitHub-mu.

2. **Di TrueNAS**, SSH masuk, buat folder mis. `/mnt/tank/apps/titipdong/`,
   taruh `docker-compose.yml` dan `.env` di sana:

   ```sh
   mkdir -p /mnt/tank/apps/titipdong && cd /mnt/tank/apps/titipdong
   # salin docker-compose.yml dan .env ke folder ini, edit .env
   docker compose up -d
   ```

3. Buka `http://<truenas-ip>:8080`, login pakai `ADMIN_EMAIL`/`ADMIN_PASSWORD`.

Data tersimpan di dua Docker volume: `pgdata` (database) dan `uploads`
(foto item/struk/KTP). Backup kedua volume ini untuk mengamankan data.

### Opsi B — build lokal (tanpa GHCR)

```sh
docker compose up -d --build
```

(di `docker-compose.yml`, comment baris `image:` dan uncomment `build: .`)

### Opsi C — pakai external Postgres

Kalau TrueNAS-mu sudah punya Postgres, set `DATABASE_URL` ke instance itu
dan hapus service `db` dari compose.

## Development lokal

```sh
# 1. Butuh Postgres jalan di localhost:5432.
createdb titipdong

# 2. Install deps + Tailwind.
npm install

# 3. Build CSS (sekali, atau `npm run build:css` tiap ganti template).
npx tailwindcss -i ./web/static/src.css -o ./web/static/app.css --minify

# 4. Run.
DATABASE_URL="postgres://postgres:a@localhost:5432/titipdong?sslmode=disable" \
ADMIN_EMAIL=admin@titipdong.local ADMIN_PASSWORD=admin12345 \
SESSION_SECRET=$(openssl rand -hex 32) \
go run ./cmd/titipdong
```

Buka `http://localhost:8080`.

## Test

```sh
go test ./...
```

## Struktur proyek

```
cmd/titipdong/         # entry point + admin bootstrap
internal/
  config/   db/   auth/   currency/
  customers/  orders/  trips/  catalog/  kyc/
  scan/   whatsapp/   web/   (handlers + templates + static)
migrations/            # SQL migrasi (di-embed ke binary)
web/static/            # htmx, alpine, tailwind app.css, manifest, sw.js
Dockerfile             # multi-stage: css -> go -> distroless
docker-compose.yml     # app + postgres + volumes
.github/workflows/     # publish ke GHCR
```

## Catatan

- WhatsApp memakai `wa.me` deep-link (kamu tap Send). Tidak butuh verifikasi
  Meta Business. Kalau mau kirim otomatis tanpa tap, ganti dengan WhatsApp
  Cloud API.
- Rate FX di-cache 24 jam di DB; kalau API down, dipakai rate cache terakhir.
- Foto disimpan sebagai file di volume `uploads/` (bukan di DB).

---

Dibuat untuk pemilik bisnis jastip sungguhan — bukan untuk finance officer. 🙏
