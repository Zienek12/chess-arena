-- Time-based search limit and resource limits for engine containers.
ALTER TABLE tournaments
    ADD COLUMN movetime_ms INTEGER CHECK (movetime_ms IS NULL OR movetime_ms > 0),
    ADD COLUMN cpu_limit NUMERIC(5, 2) NOT NULL DEFAULT 1.0 CHECK (cpu_limit > 0),
    ADD COLUMN memory_limit_mb INTEGER NOT NULL DEFAULT 256 CHECK (memory_limit_mb >= 16);

-- 001 required exactly one of depth or nodes; its constraint has a generated name.
DO $$
DECLARE
    constraint_name TEXT;
BEGIN
    SELECT conname INTO constraint_name
    FROM pg_constraint
    WHERE conrelid = 'tournaments'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%depth IS NOT NULL%nodes IS NOT NULL%';
    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE tournaments DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

ALTER TABLE tournaments
    ADD CONSTRAINT tournaments_search_limit_check CHECK (num_nonnulls(depth, nodes, movetime_ms) = 1);

-- A worker leases a game while playing it. An expired lease means the worker
-- died, and another worker may resume the game from its stored moves.
ALTER TABLE games
    ADD COLUMN lease_owner TEXT,
    ADD COLUMN lease_expires_at TIMESTAMPTZ;

CREATE INDEX games_claim_idx ON games (status, lease_expires_at);

-- Search telemetry reported by the engine, plus the arena's own measurement.
ALTER TABLE game_moves
    ADD COLUMN depth INTEGER,
    ADD COLUMN seldepth INTEGER,
    ADD COLUMN score_cp INTEGER,
    ADD COLUMN score_mate INTEGER,
    ADD COLUMN nodes BIGINT,
    ADD COLUMN nps BIGINT,
    ADD COLUMN engine_time_ms BIGINT,
    ADD COLUMN pv TEXT,
    ADD COLUMN think_time_ms INTEGER;

ALTER TABLE game_results
    ADD COLUMN pgn TEXT;
