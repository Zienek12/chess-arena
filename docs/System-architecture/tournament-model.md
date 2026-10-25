# Tournament Model

## Scope

The tournament model stores engine definitions, imported starting positions, tournament configuration, resumable games, moves, results, and execution errors.

The schema is defined in:

```text
backend/migrations/001_tournament_model.sql
```

The backend applies this migration automatically at startup when `DATABASE_URL` is set.

## Tables

- `engines` stores a stable engine name, version, Docker image, and optional UCI options.
- `position_sets` stores the source type, source name, position count, and a deterministic SHA-256 fingerprint.
- `positions` stores canonical FEN positions and their order inside a position set.
- `tournaments` stores the engine pair, position set, game count, search limit, color order, and lifecycle status.
- `games` stores one scheduled game, current FEN, ply, result, next side, retry attempt, and persisted state version.
- `game_moves` stores every UCI move and the resulting FEN.
- `game_results` stores the terminal result and termination reason.
- `game_errors` stores stage, error code, message, and retryability.

## Tournament Configuration

A tournament requires:

- two different engines,
- one imported position set,
- a positive game count,
- exactly one search limit: `depth` or `nodes`,
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

`ImportFEN` accepts one FEN per non-empty, non-comment line. `ImportPGN` reads `[FEN "..."]` tags and uses the standard starting position when a PGN has no FEN tag.

Every position is normalized and validated before storage. The position set fingerprint is:

```text
SHA256(canonical_fen_0 + "\n" + canonical_fen_1 + ...)
```

The fingerprint is used as the deterministic position-set identity and is protected by a unique database constraint.

## Idempotent Game State

A game is uniquely identified by:

```text
(tournament_id, game_number)
```

Every state update carries:

- `state_version`, which must increase monotonically,
- `idempotency_key`, which identifies the event being applied.

The PostgreSQL repository uses a transaction and row locking:

- the same idempotency key is a no-op,
- an older state version returns a stale-state error,
- a newer state replaces the current snapshot.

Moves are idempotent by `(game_id, ply)` and their own idempotency key. A repeated identical move is a no-op; a different move at the same ply is rejected.

This allows a worker to retry after a process crash without duplicating moves or moving a game backwards.

## Current Implementation Boundary

The current code provides the schema, importers, validation, in-memory idempotency reference implementation, and PostgreSQL repository. A tournament scheduler and worker still need to be added to create game rows, launch engine processes, persist move events, and advance tournament status.
