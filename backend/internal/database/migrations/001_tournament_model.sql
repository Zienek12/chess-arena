CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE engines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    image TEXT NOT NULL,
    uci_options JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, version)
);

CREATE TABLE position_sets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint CHAR(64) NOT NULL UNIQUE,
    source_type TEXT NOT NULL CHECK (source_type IN ('fen', 'pgn')),
    source_name TEXT NOT NULL,
    position_count INTEGER NOT NULL CHECK (position_count > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE positions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    position_set_id UUID NOT NULL REFERENCES position_sets(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK (ordinal >= 0),
    fen TEXT NOT NULL,
    fen_hash CHAR(64) NOT NULL,
    UNIQUE (position_set_id, ordinal),
    UNIQUE (position_set_id, fen_hash)
);

CREATE TABLE tournaments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    engine_a_id UUID NOT NULL REFERENCES engines(id),
    engine_b_id UUID NOT NULL REFERENCES engines(id),
    position_set_id UUID NOT NULL REFERENCES position_sets(id),
    game_count INTEGER NOT NULL DEFAULT 1000 CHECK (game_count > 0),
    depth INTEGER CHECK (depth IS NULL OR depth > 0),
    nodes BIGINT CHECK (nodes IS NULL OR nodes > 0),
    color_order TEXT NOT NULL DEFAULT 'alternate'
        CHECK (color_order IN ('alternate', 'engine_a_white', 'engine_b_white')),
    status TEXT NOT NULL DEFAULT 'created'
        CHECK (status IN ('created', 'queued', 'running', 'paused', 'completed', 'failed', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (engine_a_id <> engine_b_id),
    CHECK ((depth IS NOT NULL) <> (nodes IS NOT NULL))
);

CREATE TABLE games (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tournament_id UUID NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    game_number INTEGER NOT NULL CHECK (game_number > 0),
    position_id UUID NOT NULL REFERENCES positions(id),
    white_engine_id UUID NOT NULL REFERENCES engines(id),
    black_engine_id UUID NOT NULL REFERENCES engines(id),
    current_fen TEXT NOT NULL,
    ply INTEGER NOT NULL DEFAULT 0 CHECK (ply >= 0),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    result TEXT CHECK (result IN ('1-0', '0-1', '1/2-1/2', '*')),
    next_side CHAR(1) NOT NULL DEFAULT 'w' CHECK (next_side IN ('w', 'b')),
    attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
    state_version BIGINT NOT NULL DEFAULT 0 CHECK (state_version >= 0),
    idempotency_key TEXT NOT NULL,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tournament_id, game_number),
    UNIQUE (idempotency_key)
);

CREATE TABLE game_moves (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    ply INTEGER NOT NULL CHECK (ply > 0),
    uci TEXT NOT NULL,
    fen_after TEXT NOT NULL,
    engine_id UUID NOT NULL REFERENCES engines(id),
    idempotency_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (game_id, ply),
    UNIQUE (idempotency_key)
);

CREATE TABLE game_results (
    game_id UUID PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
    result TEXT NOT NULL CHECK (result IN ('1-0', '0-1', '1/2-1/2', '*')),
    termination TEXT NOT NULL,
    winner_engine_id UUID REFERENCES engines(id),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE game_errors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID REFERENCES games(id) ON DELETE CASCADE,
    stage TEXT NOT NULL,
    code TEXT NOT NULL,
    message TEXT NOT NULL,
    retryable BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX positions_set_ordinal_idx ON positions(position_set_id, ordinal);
CREATE INDEX games_status_idx ON games(status);
CREATE INDEX games_tournament_status_idx ON games(tournament_id, status);
CREATE INDEX game_errors_game_idx ON game_errors(game_id, created_at);
