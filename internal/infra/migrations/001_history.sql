CREATE TABLE IF NOT EXISTS users (
    id bigserial PRIMARY KEY,
    username varchar(50) NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE TABLE IF NOT EXISTS rooms (
    id bigserial PRIMARY KEY,
    room_number varchar(20) NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rooms_room_number ON rooms(room_number);
CREATE TABLE IF NOT EXISTS danmaku_messages (
    id bigserial PRIMARY KEY,
    room_id varchar(50) NOT NULL,
    user_id numeric(20,0) NOT NULL,
    content varchar(500) NOT NULL,
    send_time timestamptz(3) NOT NULL
);
ALTER TABLE danmaku_messages ADD COLUMN IF NOT EXISTS event_id text;
CREATE UNIQUE INDEX IF NOT EXISTS idx_danmaku_messages_event_id ON danmaku_messages(event_id);
CREATE INDEX IF NOT EXISTS idx_room_time ON danmaku_messages(room_id, send_time);
CREATE INDEX IF NOT EXISTS idx_danmaku_messages_user_id ON danmaku_messages(user_id);
