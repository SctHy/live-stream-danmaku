ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'chat'
        CHECK (category IN ('chat','gaming','coding','music','outdoor')),
    ADD COLUMN IF NOT EXISTS tags TEXT[] NOT NULL DEFAULT '{}';
CREATE INDEX IF NOT EXISTS idx_rooms_category_status ON rooms (category, status);
