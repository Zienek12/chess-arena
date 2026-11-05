# System Architecture and Infrastructure

This section documents the high-level design of the distributed chess platform, focusing on container isolation, process orchestration, and network communication between modules.

## Architectural Components

Select a topic to explore the implementation details:

- [Engine implementation](engine-implementation.md)
- [Engine internals (v1.2.0): move generation, evaluation and search](engine-internals-v.1.2.0.md)
- [Engine performance (v1.3.0): attack tables, magic bitboards and faster legality checks](engine-performance-v1.3.0.md)
- [Quiescence search (v1.4.0): removing the horizon effect](engine-quiescence-v1.4.0.md)
- [Transposition table (v1.5.0): remembering searched positions](engine-transposition-table-v1.5.0.md)
- [Tournament model](tournament-model.md)
- [Arena backend](arena-backend.md)
- [Analysis and statistics](analysis-and-statistics.md)

## Versioned engine runtime

The Play page offers, in this order:

- Every engine registered for tournaments (`POST /api/engines`), named
  `name version`, with its UCI options. A newly registered engine appears at the
  next new game, without a restart.
- Docker images configured with `CHESS_ENGINE_IMAGES`, a comma-separated
  allowlist in the form `version=image`.
- `local`, which uses `CHESS_ENGINE_PATH` or the repository's debug binary. It
  is listed only when that file exists.

`CHESS_ENGINE_VERSION` names the engine selected on connect; without it, or
when it is not available, the first engine is selected. If an engine fails to
start, the error message carries the list so another one can be chosen.

The selected version is sent in the WebSocket `state` message and can be
changed by sending `{"type":"reset","engine":"<version>"}`. Docker-backed
versions are started in isolated containers (see [Arena backend](arena-backend.md#engine-sandbox)), so the backend process
needs access to the Docker CLI and daemon. The root `docker-compose.yml`
provides PostgreSQL and a backend service for local platform development.
