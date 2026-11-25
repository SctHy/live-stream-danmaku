ALTER TABLE danmaku_messages
 ADD COLUMN color text NOT NULL DEFAULT '#ffffff' CHECK(color IN ('#ffffff','#34d399','#38bdf8','#facc15','#fb7185','#c084fc')),
 ADD COLUMN mode text NOT NULL DEFAULT 'scroll' CHECK(mode IN ('scroll','top','bottom'));
ALTER TABLE users ADD COLUMN earnings bigint NOT NULL DEFAULT 0 CHECK(earnings>=0);
ALTER TABLE rooms ADD COLUMN cover_theme text NOT NULL DEFAULT 'zinc'
 CHECK(cover_theme IN ('zinc','emerald','ocean','amber','violet','rose'));
CREATE TABLE room_follows (
 user_id numeric(20,0) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 room_slug text NOT NULL REFERENCES rooms(slug) ON DELETE CASCADE,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id,room_slug)
);
CREATE INDEX room_follows_slug ON room_follows(room_slug);
CREATE INDEX danmaku_replay_legacy ON danmaku_messages(room_id,send_time) INCLUDE(event_id)
 WHERE video_offset_ms IS NULL;
CREATE INDEX danmaku_replay_count ON danmaku_messages(room_id,event_id) WHERE video_offset_ms IS NOT NULL;

ALTER TABLE room_replays ADD COLUMN visible_count bigint;
-- Invalidate once per affected replay, including late Kafka batches and withdrawals.
CREATE FUNCTION invalidate_replay_counts() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 -- Lock in ID order, including live/processing rows racing the ready transition.
 WITH affected AS MATERIALIZED (
  SELECT id,status FROM room_replays WHERE room_id IN (SELECT room_id FROM inserted)
  ORDER BY id FOR UPDATE
 )
 UPDATE room_replays SET visible_count=NULL FROM affected
 WHERE room_replays.id=affected.id AND affected.status='ready';
 RETURN NULL;
END $$;
CREATE TRIGGER replay_messages_inserted AFTER INSERT ON danmaku_messages
 REFERENCING NEW TABLE AS inserted FOR EACH STATEMENT EXECUTE FUNCTION invalidate_replay_counts();
CREATE TRIGGER replay_withdrawals_inserted AFTER INSERT ON danmaku_withdrawals
 REFERENCING NEW TABLE AS inserted FOR EACH STATEMENT EXECUTE FUNCTION invalidate_replay_counts();

CREATE FUNCTION ready_replay_count(replay bigint) RETURNS bigint LANGUAGE plpgsql AS $$
DECLARE p room_replays; cacheable boolean;
BEGIN
 SELECT * INTO p FROM room_replays WHERE id=replay FOR UPDATE SKIP LOCKED;
 cacheable := FOUND;
 IF NOT cacheable THEN
  SELECT * INTO p FROM room_replays WHERE id=replay;
 END IF;
 IF p.visible_count IS NULL THEN
  SELECT count(*) INTO p.visible_count FROM danmaku_messages m WHERE m.room_id=p.room_id
   AND (m.video_offset_ms IS NOT NULL OR m.send_time BETWEEN p.started_at AND coalesce(p.ended_at,now()))
   AND NOT EXISTS(SELECT FROM danmaku_withdrawals w WHERE w.room_id=m.room_id AND w.event_id=m.event_id);
  IF cacheable THEN
   UPDATE room_replays SET visible_count=p.visible_count WHERE id=replay;
  END IF;
 END IF;
 RETURN p.visible_count;
END $$;
