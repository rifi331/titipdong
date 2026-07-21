-- Order status v2: unified lifecycle enum replacing the old 5-step pipeline
-- (dicari/ketemu/dibeli/dibayar/diantar) + separate `paid` boolean.
--
-- New 9-value enum tracks the full lifecycle:
--   pending_confirmation -> accepted -> waiting_for_payment -> paid -> delivery -> finished
--   with side exits: rejected, buyer_cancelled, seller_cancelled
--
-- Payment detail columns (paid_at, payment_method, paid_amount, payment_ref)
-- are filled when status transitions to 'paid', for audit / dispute resolution.

-- 1. Payment detail columns (nullable, filled on status=paid)
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS paid_at        TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS payment_method TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS paid_amount    NUMERIC(14,2),
    ADD COLUMN IF NOT EXISTS payment_ref    TEXT NOT NULL DEFAULT '';

-- 2. New enum type
DO $$ BEGIN
    CREATE TYPE order_status_v2 AS ENUM (
        'pending_confirmation', 'accepted', 'rejected',
        'buyer_cancelled', 'waiting_for_payment', 'seller_cancelled',
        'paid', 'delivery', 'finished'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- 3. Add new status column (nullable during migration)
ALTER TABLE orders ADD COLUMN IF NOT EXISTS status_v2 order_status_v2;

-- 4. Backfill from old status + paid boolean.
--    Order matters: paid wins over everything (since paid=true could coexist
--    with any old pipeline step).
UPDATE orders SET status_v2 = 'paid'                WHERE (status = 'dibayar' OR paid = true) AND status_v2 IS NULL;
UPDATE orders SET status_v2 = 'delivery'            WHERE status = 'diantar' AND status_v2 IS NULL;
UPDATE orders SET status_v2 = 'waiting_for_payment' WHERE status = 'dibeli'   AND status_v2 IS NULL;
UPDATE orders SET status_v2 = 'accepted'            WHERE status IN ('dicari','ketemu') AND status_v2 IS NULL;
-- Anything still NULL (shouldn't happen, but safety net) -> accepted
UPDATE orders SET status_v2 = 'accepted'            WHERE status_v2 IS NULL;
-- Backfill paid_at for migrated paid orders (use created_at as best-effort proxy)
UPDATE orders SET paid_at = created_at              WHERE status_v2 = 'paid' AND paid_at IS NULL;

-- 5. Drop old columns and type
ALTER TABLE orders DROP COLUMN IF EXISTS paid;
ALTER TABLE orders DROP COLUMN IF EXISTS status;
DROP TYPE IF EXISTS order_status;

-- 6. Promote status_v2 to NOT NULL and rename column + type
ALTER TABLE orders ALTER COLUMN status_v2 SET NOT NULL;
ALTER TABLE orders RENAME COLUMN status_v2 TO status;
ALTER TYPE order_status_v2 RENAME TO order_status;

-- 7. Rebuild status index
DROP INDEX IF EXISTS idx_orders_status;
CREATE INDEX idx_orders_status ON orders(status);
