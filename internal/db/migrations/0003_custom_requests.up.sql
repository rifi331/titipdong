-- Extend buyer_requests to support custom-item requests (not just catalog items).
--
-- Two request types now coexist in buyer_requests:
--   - catalog:    buyer taps "Mau Ini!" on a catalog item (catalog_item_id set, item_* copied)
--   - custom:     buyer describes an item themselves     (catalog_item_id NULL, item_* filled by buyer)
--
-- The jastiper sets the fee on accept using one of:
--   - percent:  fee = est_price_foreign × fx × fee_percent/100
--   - per_kg:   fee = est_weight_kg × fee_per_kg_idr
-- The converted order's selling_price_idr is set accordingly.

-- Nullable catalog_item_id (was NOT NULL): custom requests have no catalog item.
ALTER TABLE buyer_requests ALTER COLUMN catalog_item_id DROP NOT NULL;

-- Item snapshot fields (used for both catalog and custom requests so the
-- request remains self-contained even if the catalog item is later deleted).
ALTER TABLE buyer_requests
    ADD COLUMN IF NOT EXISTS item_title          TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS item_description    TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS item_currency       TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS item_est_price      NUMERIC(14,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS item_origin         TEXT NOT NULL DEFAULT '',   -- country/city
    ADD COLUMN IF NOT EXISTS item_est_weight_kg  NUMERIC(10,3) NOT NULL DEFAULT 0;

-- Fee model set by the jastiper on accept.
DO $$ BEGIN
    CREATE TYPE fee_model AS ENUM ('percent', 'per_kg');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

ALTER TABLE buyer_requests
    ADD COLUMN IF NOT EXISTS fee_model      fee_model,
    ADD COLUMN IF NOT EXISTS fee_percent    NUMERIC(5,2),     -- e.g. 20.00
    ADD COLUMN IF NOT EXISTS fee_per_kg_idr NUMERIC(14,2);    -- e.g. 50000

-- Backfill item_* snapshot for existing catalog requests so the new columns
-- aren't all empty for rows that already exist.
UPDATE buyer_requests r
SET item_title       = c.title,
    item_description = c.description,
    item_currency    = c.currency,
    item_est_price   = c.est_price_foreign
FROM catalog_items c
WHERE r.catalog_item_id = c.id AND r.item_title = '';
