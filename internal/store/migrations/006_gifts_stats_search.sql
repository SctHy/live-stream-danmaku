CREATE TABLE gift_orders (
    id BIGSERIAL PRIMARY KEY,
    client_order_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    room_slug TEXT NOT NULL,
    sender_id NUMERIC(20,0) NOT NULL REFERENCES users(id),
    sender_name TEXT NOT NULL,
    gift_code TEXT NOT NULL,
    gift_name TEXT NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_price BIGINT NOT NULL CHECK (unit_price > 0),
    total_cost BIGINT NOT NULL CHECK (total_cost > 0),
    message TEXT NOT NULL DEFAULT '',
    pinned_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    pending BOOLEAN NOT NULL DEFAULT true,
    UNIQUE(sender_id, client_order_id)
);
CREATE INDEX gift_room_recent ON gift_orders(room_id,id DESC);
CREATE INDEX gift_room_pins ON gift_orders(room_id,pinned_until DESC);
CREATE INDEX gift_sender_recent ON gift_orders(sender_id,id DESC);
CREATE INDEX gift_pending ON gift_orders(room_id,id) WHERE pending;

CREATE TABLE daily_checkins (
    user_id NUMERIC(20,0) NOT NULL REFERENCES users(id),
    claim_date DATE NOT NULL,
    amount BIGINT NOT NULL CHECK(amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(user_id, claim_date)
);
CREATE INDEX danmaku_room_sender_cursor ON danmaku_messages(room_id,user_id,id DESC);
