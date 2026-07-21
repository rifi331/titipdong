-- Trips v2: richer trip metadata for jastiper planning.
-- Replaces the simple (name, country, start_date, end_date, active/closed)
-- with full logistics: destination city, order cutoff, estimated delivery,
-- weight/slot limits, notes, and a 3-stage status lifecycle.

-- 1. Rename existing columns to English spec names
ALTER TABLE trips RENAME COLUMN name TO title;
ALTER TABLE trips RENAME COLUMN country TO destination_country;
ALTER TABLE trips RENAME COLUMN start_date TO departure_date;
ALTER TABLE trips RENAME COLUMN end_date TO return_date;

-- 2. Add new columns
ALTER TABLE trips
    ADD COLUMN IF NOT EXISTS destination_city     TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS order_cutoff_at      TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS estimated_delivery   DATE,
    ADD COLUMN IF NOT EXISTS max_weight_kg        NUMERIC(10,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS used_weight_kg       NUMERIC(10,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_item_slots       INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS notes                TEXT NOT NULL DEFAULT '';

-- 3. New status enum (3-stage lifecycle)
DO $$ BEGIN
    CREATE TYPE trip_status_v2 AS ENUM ('on_plan', 'at_destination_country', 'in_home_country');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- 4. Add new status column + backfill
ALTER TABLE trips ADD COLUMN IF NOT EXISTS status_v2 trip_status_v2;
UPDATE trips SET status_v2 = 'on_plan'            WHERE status = 'active' AND status_v2 IS NULL;
UPDATE trips SET status_v2 = 'in_home_country'    WHERE status = 'closed' AND status_v2 IS NULL;
UPDATE trips SET status_v2 = 'on_plan'            WHERE status_v2 IS NULL;

-- 5. Drop old status column + type
ALTER TABLE trips DROP COLUMN IF EXISTS status;
DROP TYPE IF EXISTS trip_status;

-- 6. Promote new column
ALTER TABLE trips ALTER COLUMN status_v2 SET NOT NULL;
ALTER TABLE trips RENAME COLUMN status_v2 TO status;

-- 7. Rename the type to match what Go code expects
ALTER TYPE trip_status_v2 RENAME TO trip_status;
