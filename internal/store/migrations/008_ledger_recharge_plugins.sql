CREATE TABLE wallet_ledger (
 id BIGSERIAL PRIMARY KEY,
 user_id NUMERIC(20,0) NOT NULL REFERENCES users(id),
 biz_type TEXT NOT NULL,
 biz_id TEXT NOT NULL,
 delta BIGINT NOT NULL CHECK(delta <> 0),
 balance_before BIGINT NOT NULL CHECK(balance_before >= 0),
 balance_after BIGINT NOT NULL CHECK(balance_after >= 0 AND balance_before + delta = balance_after),
 meta JSONB NOT NULL DEFAULT '{}',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(user_id,biz_type,biz_id)
);
CREATE INDEX wallet_ledger_recent ON wallet_ledger(user_id,id DESC);
CREATE FUNCTION reject_ledger_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'wallet ledger is append-only'; END $$;
CREATE TRIGGER ledger_immutable BEFORE UPDATE OR DELETE OR TRUNCATE ON wallet_ledger
 FOR EACH STATEMENT EXECUTE FUNCTION reject_ledger_change();

CREATE TABLE recharge_orders (
 id BIGSERIAL PRIMARY KEY,
 order_no TEXT UNIQUE NOT NULL,
 client_order_id TEXT NOT NULL,
 user_id NUMERIC(20,0) NOT NULL REFERENCES users(id),
 provider TEXT NOT NULL,
 package_code TEXT NOT NULL,
 amount_cents BIGINT NOT NULL CHECK(amount_cents >= 0),
 coins BIGINT NOT NULL CHECK(coins > 0),
 status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','paid','closed','expired')),
 provider_trade_no TEXT UNIQUE,
 expires_at TIMESTAMPTZ NOT NULL,
 paid_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(user_id,client_order_id)
);
CREATE INDEX recharge_orders_recent ON recharge_orders(user_id,id DESC);
ALTER TABLE gift_orders
 ADD COLUMN IF NOT EXISTS effect_type TEXT NOT NULL DEFAULT 'standard',
 ADD COLUMN IF NOT EXISTS combo_key TEXT NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS combo_count INT NOT NULL DEFAULT 1 CHECK(combo_count >= 1);
UPDATE gift_orders SET effect_type='superchat' WHERE pinned_until IS NOT NULL;
