# Tournament Model

## Scope

The tournament model stores engine definitions, imported starting positions, tournament configuration, resumable games, moves, results, and execution errors.

The schema is defined by the migrations in:

```text
backend/internal/database/migrations/
```

The backend applies pending migrations automatically at startup when `DATABASE_URL` is set; see [arena-backend.md](arena-backend.md#database-migrations). The domain model lives in `backend/internal/tournament` and the repository in `backend/internal/store`.

## Tables

- `engines` stores a stable engine name, version, Docker image, and optional UCI options.
- `position_sets` stores the source type, source name, position count, and a deterministic SHA-256 fingerprint.
- `positions` stores canonical FEN positions and their order inside a position set.
- `tournaments` stores the engine pair, position set, game count, search limit, color order, container CPU and memory limits, and lifecycle status.
- `games` stores one scheduled game, current FEN, ply, result, next side, retry attempt, state version, and the worker lease (`lease_owner`, `lease_expires_at`).
- `game_moves` stores every UCI move, the resulting FEN, the engine's search telemetry (`depth`, `seldepth`, `score_cp`, `score_mate`, `nodes`, `nps`, `engine_time_ms`, `pv`) and the arena-measured `think_time_ms`.
- `game_results` stores the terminal result, termination reason, and the game's PGN.
- `game_errors` stores stage, error code, message, and retryability.

## Tournament Configuration

A tournament requires:

- two different engines,
- one imported position set,
- a positive game count,
- exactly one search limit: `movetime_ms`, `depth` or `nodes`,
- CPU and memory limits for engine containers (defaults 1.0 CPU and 256 MB),
- a color policy: `alternate`, `engine_a_white`, or `engine_b_white`.

The default game count is `1000`.

Tournament lifecycle values are:

```text
created -> queued -> running -> completed
                         |-> paused
                         |-> failed
                         |-> cancelled
```

## Position Import

`ImportFEN` accepts one FEN per non-empty, non-comment line. `ImportPGN` takes the final position of every game: a game with only a `[FEN "..."]` tag yields that position, and a game with moves, such as an opening line, yields the position after its moves. Every game in a PGN file needs a tag section, as the PGN standard requires.

Every position is normalized and validated before storage, and duplicate positions inside one import are dropped. The position set fingerprint is:

```text
SHA256(canonical_fen_0 + "\n" + canonical_fen_1 + ...)
```

The fingerprint is used as the deterministic position-set identity and is protected by a unique database constraint.

## Game Lifecycle and Idempotency

A game is uniquely identified by `(tournament_id, game_number)`.

A worker leases a game when it claims it (`queued` → `running`). Every stored move renews the lease, and every write checks that the worker still owns it, so a game that was cancelled or taken over by another worker cannot be modified by the old one.

Moves are idempotent by `(game_id, ply)` and an idempotency key derived from the tournament, game number, ply and move. A repeated identical move is a no-op; a different move at the same ply is rejected. When a worker dies, its lease expires and another worker resumes the game by replaying the stored moves, without duplicating moves or moving the game backwards.

See [arena-backend.md](arena-backend.md) for how workers play games and handle failures.
