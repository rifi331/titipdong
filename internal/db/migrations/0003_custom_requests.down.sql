ALTER TABLE buyer_requests
    DROP COLUMN IF EXISTS fee_per_kg_idr,
    DROP COLUMN IF EXISTS fee_percent,
    DROP COLUMN IF EXISTS fee_model,
    DROP COLUMN IF EXISTS item_est_weight_kg,
    DROP COLUMN IF EXISTS item_origin,
    DROP COLUMN IF EXISTS item_est_price,
    DROP COLUMN IF EXISTS item_currency,
    DROP COLUMN IF EXISTS item_description,
    DROP COLUMN IF EXISTS item_title;

ALTER TABLE buyer_requests ALTER COLUMN catalog_item_id SET NOT NULL;

DROP TYPE IF EXISTS fee_model;
