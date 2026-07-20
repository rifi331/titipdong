-- TitipDong core schema.
-- Roles: buyer (default on signup) -> jastiper (after admin-approved KYC) -> admin (operator).

-- Roles enum --------------------------------------------------------------
DO $$ BEGIN
    CREATE TYPE user_role AS ENUM ('buyer', 'jastiper', 'admin');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Application status for the "Become a Jastiper" KYC flow -----------------
DO $$ BEGIN
    CREATE TYPE application_status AS ENUM ('pending', 'approved', 'rejected');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- Order status pipeline: Dicari -> Ketemu -> Dibeli -> Dibayar -> Diantar --
DO $$ BEGIN
    CREATE TYPE order_status AS ENUM ('dicari', 'ketemu', 'dibeli', 'dibayar', 'diantar');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE trip_status AS ENUM ('active', 'closed');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE catalog_status AS ENUM ('active', 'archived');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- users --------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id              BIGSERIAL PRIMARY KEY,
    email           TEXT NOT NULL UNIQUE,
    password_hash   BYTEA NOT NULL,
    role            user_role NOT NULL DEFAULT 'buyer',
    display_name    TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- jastiper_applications (the KYC) -----------------------------------------
CREATE TABLE IF NOT EXISTS jastiper_applications (
    id                   BIGSERIAL PRIMARY KEY,
    user_id              BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    full_name            TEXT NOT NULL,
    ktp_number           TEXT NOT NULL,
    ktp_photo_path       TEXT NOT NULL DEFAULT '',
    phone                TEXT NOT NULL DEFAULT '',
    city                 TEXT NOT NULL DEFAULT '',
    status               application_status NOT NULL DEFAULT 'pending',
    reviewed_by_user_id  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at          TIMESTAMPTZ,
    admin_note           TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_jastiper_applications_status ON jastiper_applications(status);
CREATE INDEX IF NOT EXISTS idx_jastiper_applications_user   ON jastiper_applications(user_id);

-- customers ----------------------------------------------------------------
CREATE TABLE IF NOT EXISTS customers (
    id              BIGSERIAL PRIMARY KEY,
    owner_user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    whatsapp        TEXT NOT NULL DEFAULT '',   -- E.164, digits only, used by wa.me
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_customers_owner ON customers(owner_user_id);

-- trips --------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS trips (
    id              BIGSERIAL PRIMARY KEY,
    owner_user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,            -- e.g. "Tokyo Trip Juli 2026"
    country         TEXT NOT NULL DEFAULT '',
    currency        TEXT NOT NULL DEFAULT 'JPY',
    start_date      DATE,
    end_date        DATE,
    status          trip_status NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_trips_owner ON trips(owner_user_id);

-- orders -------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS orders (
    id                   BIGSERIAL PRIMARY KEY,
    owner_user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    customer_id          BIGINT REFERENCES customers(id) ON DELETE SET NULL,
    trip_id              BIGINT REFERENCES trips(id) ON DELETE SET NULL,
    item_name            TEXT NOT NULL,
    source_store         TEXT NOT NULL DEFAULT '',  -- Don Quijote, Matsumoto Kiyoshi, ...
    currency             TEXT NOT NULL DEFAULT 'JPY',
    amount_foreign       NUMERIC(14,2) NOT NULL DEFAULT 0,
    markup_pct           NUMERIC(5,2) NOT NULL DEFAULT 0,
    fx_rate_snapshot     NUMERIC(18,6) NOT NULL DEFAULT 1,  -- 1 unit foreign => N IDR at order time
    selling_price_idr    NUMERIC(14,2) NOT NULL DEFAULT 0,
    status               order_status NOT NULL DEFAULT 'dicari',
    paid                 BOOLEAN NOT NULL DEFAULT FALSE,
    photo_path           TEXT NOT NULL DEFAULT '',
    note                 TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_orders_owner   ON orders(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_orders_trip    ON orders(trip_id);
CREATE INDEX IF NOT EXISTS idx_orders_status  ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders(customer_id);

-- catalog_items (browseable by buyers) -------------------------------------
CREATE TABLE IF NOT EXISTS catalog_items (
    id                  BIGSERIAL PRIMARY KEY,
    jastiper_user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title               TEXT NOT NULL,
    description         TEXT NOT NULL DEFAULT '',
    est_price_foreign   NUMERIC(14,2) NOT NULL DEFAULT 0,
    currency            TEXT NOT NULL DEFAULT 'JPY',
    photo_path          TEXT NOT NULL DEFAULT '',
    status              catalog_status NOT NULL DEFAULT 'active',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_catalog_status ON catalog_items(status);

-- fx_rates (cache; quote always IDR) ---------------------------------------
CREATE TABLE IF NOT EXISTS fx_rates (
    base        TEXT NOT NULL,             -- JPY, KRW, USD ...
    quote       TEXT NOT NULL DEFAULT 'IDR',
    rate        NUMERIC(18,6) NOT NULL,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (base, quote)
);
