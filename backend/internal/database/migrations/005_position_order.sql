-- Order in which a tournament uses its starting positions. Existing
-- tournaments used the set's order; new ones default to a random order.
ALTER TABLE tournaments
    ADD COLUMN position_order TEXT NOT NULL DEFAULT 'sequential'
        CHECK (position_order IN ('random', 'sequential'));
