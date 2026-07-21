-- Down migration: best-effort restore of old schema.
-- Some new statuses (rejected, buyer_cancelled, seller_cancelled,
-- pending_confirmation, finished) have no clean old equivalent and are
-- mapped to the closest old value.

-- 1. Recreate old enum
DO $$ BEGIN
    CREATE TYPE order_status AS ENUM ('dicari', 'ketemu', 'dibeli', 'dibayar', 'diantar');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- 2. Rename current column out of the way
ALTER TABLE orders RENAME COLUMN status TO status_v2;

-- 3. Add back old column
ALTER TABLE orders ADD COLUMN status order_status NOT NULL DEFAULT 'dicari';
ALTER TABLE orders ADD COLUMN paid BOOLEAN NOT NULL DEFAULT FALSE;

-- 4. Backfill old from new
UPDATE orders SET status = 'dibayar', paid = true WHERE status_v2 = 'paid';
UPDATE orders SET status = 'dibayar', paid = true WHERE status_v2 = 'delivery';
UPDATE orders SET status = 'dibayar', paid = true WHERE status_v2 = 'finished';
UPDATE orders SET status = 'dibeli'  WHERE status_v2 = 'waiting_for_payment';
UPDATE orders SET status = 'ketemu'  WHERE status_v2 = 'accepted';
UPDATE orders SET status = 'dicari'  WHERE status_v2 IN ('pending_confirmation','rejected','buyer_cancelled','seller_cancelled');

-- 5. Drop new enum column + type + payment columns
ALTER TABLE orders DROP COLUMN status_v2;
DROP TYPE IF EXISTS order_status_v2;
ALTER TABLE orders
    DROP COLUMN IF EXISTS paid_at,
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS paid_amount,
    DROP COLUMN IF EXISTS payment_ref;

-- 6. Rebuild index
DROP INDEX IF EXISTS idx_orders_status;
CREATE INDEX idx_orders_status ON orders(status);
