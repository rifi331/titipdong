-- Down migration: revert trips to v1 schema.
-- Best-effort: maps new 3-stage statuses back to active/closed.

DO $$ BEGIN
    CREATE TYPE trip_status_old AS ENUM ('active', 'closed');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE trips ADD COLUMN status_old trip_status_old NOT NULL DEFAULT 'active';
UPDATE trips SET status_old = 'active'         WHERE status = 'on_plan';
UPDATE trips SET status_old = 'closed'         WHERE status IN ('at_destination_country', 'in_home_country');

ALTER TABLE trips DROP COLUMN IF EXISTS status;
DROP TYPE IF EXISTS trip_status;
ALTER TYPE trip_status_old RENAME TO trip_status;
ALTER TABLE trips RENAME COLUMN status_old TO status;

ALTER TABLE trips
    DROP COLUMN IF EXISTS notes,
    DROP COLUMN IF EXISTS max_item_slots,
    DROP COLUMN IF EXISTS used_weight_kg,
    DROP COLUMN IF EXISTS max_weight_kg,
    DROP COLUMN IF EXISTS estimated_delivery,
    DROP COLUMN IF EXISTS order_cutoff_at,
    DROP COLUMN IF EXISTS destination_city;

ALTER TABLE trips RENAME COLUMN title TO name;
ALTER TABLE trips RENAME COLUMN destination_country TO country;
ALTER TABLE trips RENAME COLUMN departure_date TO start_date;
ALTER TABLE trips RENAME COLUMN return_date TO end_date;
