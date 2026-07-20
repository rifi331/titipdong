-- Anonymous buyer requests: a buyer browses the public catalog and taps
-- "Mau Ini!" to express interest without signing up. The jastiper reviews
-- the request in their dashboard, optionally chats via WhatsApp, then
-- accepts (converting it into a real order + customer) or rejects it.

DO $$ BEGIN
    CREATE TYPE request_status AS ENUM ('pending', 'accepted', 'rejected');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS buyer_requests (
    id                BIGSERIAL PRIMARY KEY,
    catalog_item_id   BIGINT NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
    jastiper_user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    buyer_name        TEXT NOT NULL,
    buyer_whatsapp    TEXT NOT NULL,            -- digits only, E.164-ish
    buyer_note        TEXT NOT NULL DEFAULT '',  -- quantity/variant/free text
    status            request_status NOT NULL DEFAULT 'pending',
    -- Set when the jastiper accepts; points to the rows created on conversion.
    converted_order_id    BIGINT REFERENCES orders(id) ON DELETE SET NULL,
    converted_customer_id BIGINT REFERENCES customers(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_buyer_requests_jastiper ON buyer_requests(jastiper_user_id);
CREATE INDEX IF NOT EXISTS idx_buyer_requests_status   ON buyer_requests(status);

-- Spam throttle is enforced at the application layer (Submit checks for a
-- pending duplicate within the last hour). Postgres partial indexes can't
-- use now() in the predicate because it's STABLE, not IMMUTABLE.
