ALTER TABLE rooms ADD COLUMN IF NOT EXISTS stream_key text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS stream_mode text NOT NULL DEFAULT 'external' CHECK (stream_mode IN ('external','obs'));

CREATE TABLE room_replays (
    id bigserial PRIMARY KEY,
    room_slug text NOT NULL REFERENCES rooms(slug) ON DELETE CASCADE,
    session_id text NOT NULL,
    room_id text NOT NULL UNIQUE,
    title text NOT NULL,
    status text NOT NULL DEFAULT 'live' CHECK (status IN ('live','processing','ready','failed')),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    duration_ms bigint NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
    video_path text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON room_replays(room_slug,id DESC);
CREATE INDEX ON room_replays(room_id,started_at DESC);
CREATE UNIQUE INDEX ON room_replays(video_path) WHERE video_path <> '';
ALTER TABLE danmaku_messages ADD COLUMN IF NOT EXISTS video_offset_ms bigint CHECK (video_offset_ms IS NULL OR video_offset_ms >= 0);
CREATE INDEX IF NOT EXISTS idx_danmaku_replay_timeline ON danmaku_messages(room_id,video_offset_ms,id) WHERE video_offset_ms IS NOT NULL;
