CREATE TABLE users (
    id numeric(20,0) PRIMARY KEY,
    username text UNIQUE NOT NULL,
    nickname text NOT NULL,
    password_hash text NOT NULL,
    balance bigint NOT NULL DEFAULT 1000 CHECK (balance >= 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rooms (
    slug text PRIMARY KEY CHECK (slug ~ '^[a-zA-Z0-9_-]{1,24}$'),
    owner_id numeric(20,0) NOT NULL REFERENCES users(id),
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    notice text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'live' CHECK (status IN ('live','offline')),
    active_session text NOT NULL DEFAULT 'live',
    bilibili_room_id text NOT NULL DEFAULT '',
    slow_mode_seconds int NOT NULL DEFAULT 0 CHECK (slow_mode_seconds BETWEEN 0 AND 300),
    mute_all boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE room_moderators (
    room_slug text NOT NULL REFERENCES rooms(slug) ON DELETE CASCADE,
    user_id numeric(20,0) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    granted_by numeric(20,0) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (room_slug,user_id)
);

CREATE TABLE moderation_audits (
    id bigserial PRIMARY KEY,
    room_id text NOT NULL,
    actor_id numeric(20,0) NOT NULL,
    action text NOT NULL,
    target_user_id numeric(20,0) NOT NULL DEFAULT 0,
    target_event_id text NOT NULL DEFAULT '',
    detail text NOT NULL DEFAULT '',
    session_slug text NOT NULL DEFAULT '',
    pending boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX moderation_audits_room_cursor ON moderation_audits(room_id,id DESC);

ALTER TABLE danmaku_messages ADD COLUMN nickname text NOT NULL DEFAULT '';
CREATE INDEX moderation_pending ON moderation_audits(room_id, id) WHERE pending;
