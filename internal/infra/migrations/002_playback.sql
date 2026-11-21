ALTER TABLE danmaku_messages ADD COLUMN video_offset_ms bigint;
CREATE INDEX idx_danmaku_playback ON danmaku_messages(room_id,video_offset_ms,id)
WHERE video_offset_ms IS NOT NULL;
CREATE TABLE danmaku_withdrawals (
    room_id text NOT NULL, event_id text NOT NULL, PRIMARY KEY(room_id,event_id)
);
