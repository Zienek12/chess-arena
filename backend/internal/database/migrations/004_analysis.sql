-- Analysis of finished games by a reference engine (Stockfish). A worker
-- leases a game like the arena workers do, and an expired lease means the
-- analysis can be picked up again.
CREATE TABLE game_analyses (
    game_id UUID PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed')),
    engine TEXT,
    nodes BIGINT,
    attempt INTEGER NOT NULL DEFAULT 1,
    lease_owner TEXT,
    lease_expires_at TIMESTAMPTZ,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX game_analyses_claim_idx ON game_analyses (status, lease_expires_at);

-- The verdict on every played move. Evaluations are from White's point of
-- view in centipawns; *_mate is set when a forced mate was found.
CREATE TABLE move_analyses (
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    ply INTEGER NOT NULL CHECK (ply > 0),
    best_move TEXT,
    best_pv TEXT,
    depth INTEGER NOT NULL,
    before_cp INTEGER NOT NULL,
    before_mate INTEGER,
    after_cp INTEGER NOT NULL,
    after_mate INTEGER,
    cp_loss INTEGER NOT NULL CHECK (cp_loss >= 0),
    accuracy DOUBLE PRECISION NOT NULL CHECK (accuracy BETWEEN 0 AND 100),
    classification TEXT NOT NULL CHECK (classification IN ('best', 'good', 'inaccuracy', 'mistake', 'blunder')),
    PRIMARY KEY (game_id, ply)
);

-- Lets the leaderboard and tournament pages find finished games quickly.
CREATE INDEX games_finished_idx ON games (status, finished_at DESC);
