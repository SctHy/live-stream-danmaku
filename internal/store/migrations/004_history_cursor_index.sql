CREATE INDEX IF NOT EXISTS idx_danmaku_messages_room_id_desc ON danmaku_messages(room_id, id DESC);
