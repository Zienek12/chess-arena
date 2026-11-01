# Arena Backend

The Go backend in `backend/` runs tournaments between engines. It stores everything in PostgreSQL, starts every engine in its own Docker container with resource limits, validates every move and enforces the clock.

## Packages

Each package under `backend/internal/` has one responsibility. Arrows show imports.

| Package | Responsibility |
| --- | --- |
| `config` | Reads settings from environment variables. |
| `database` | Opens the PostgreSQL pool and applies versioned migrations from `database/migrations/`. |
| `tournament` | Domain model: tournament configuration and validation, FEN/PGN position import, game schedule. |
| `rules` | Chess rules (via `github.com/corentings/chess/v2`): move validation, results, PGN export. |
| `uci` | UCI client with a deadline on every call, engine error detection and `info` telemetry parsing. |
| `sandbox` | Starts engines: Docker containers with limits, or local processes for development. |
| `store` | PostgreSQL repository: engines, position sets, tournaments, game lifecycle, read models. |
| `arena` | Plays a game between two engines (`Match`) and runs the queue (`Worker`). |
| `api` | REST API for the frontend. |
| `play` | Human-versus-engine WebSocket at `/ws`, used by the current frontend. |

```text
main ──► config, database, api, arena, play
api ──► store, tournament
arena ──► store, sandbox, uci, rules, tournament
store ──► tournament, uci
play ──► sandbox, uci, rules
tournament ──► rules
```

`rules`, `uci` and `sandbox` do not depend on the database, so they can be tested and reused on their own.

## How a Tournament Runs

1. A client registers engines (`POST /api/engines`) and imports a position set (`POST /api/position-sets`).
2. `POST /api/tournaments` validates the configuration and, in one transaction, stores the tournament as `queued` and schedules every game. With `alternate` colors, games 1–2 share position 1 with colors swapped, games 3–4 share position 2, and so on.
3. Each worker claims the next game with `SELECT … FOR UPDATE SKIP LOCKED`, so several workers or backend instances never play the same game. The claim is a lease that expires after 5 minutes unless renewed.
4. The worker starts both engines as containers, performs the UCI handshake and plays the game:
   - every move must arrive within `movetime + ARENA_MOVE_MARGIN_MS`, or within `ARENA_SEARCH_TIMEOUT_MS` for `depth` and `nodes` limits;
   - every move is validated by `rules`; checkmate, stalemate, threefold repetition, the fifty-move rule and insufficient material end the game;
   - each move is stored with its telemetry in the same transaction that renews the lease.
5. When the game ends, the result, termination and PGN are stored. After the last game the tournament becomes `completed`.

### Engine Faults and Infrastructure Faults

| Situation | Outcome |
| --- | --- |
| No answer before the deadline | Loss by `time_forfeit` |
| Illegal or malformed move (including `bestmove 0000` in a live position) | Loss by `illegal_move` |
| Engine crashes or reports `info string error …` during a search | Loss by `engine_error` |
| Engine container cannot start or fails the handshake | Game is requeued and retried; failed after 3 attempts |
| Worker dies mid-game | Lease expires; another worker resumes the game from its stored moves |
| Backend shuts down | The current game is requeued immediately |
| Tournament is cancelled | Queued games are cancelled; running games lose their lease and stop at the next move |

## Engine Sandbox

`sandbox.DockerLauncher` runs:

```text
docker run --rm --interactive --init --network none --read-only --cap-drop ALL
  --security-opt no-new-privileges --pids-limit 64
  --cpus <cpus> --memory <memory_mb>m --memory-swap <memory_mb>m <image>
```

The image is pulled first when it is missing. `cpus` and `memory_mb` come from the tournament (defaults 1.0 and 256 MB). The swap limit equals the memory limit, so memory is a hard limit.

The backend drives containers through the Docker CLI. On Docker Desktop for Windows, starting a container takes about 9 seconds, and the CLI adds latency spikes of up to about 1.5 seconds per message while other containers start. Short `movetime` limits then cause time forfeits that are not the engine's fault. Use a larger `ARENA_MOVE_MARGIN_MS` there, and measure on the target Linux host before relying on tight time controls.

## Database Migrations

Migrations are SQL files named `NNN_name.sql` in `backend/internal/database/migrations/`, embedded into the binary. At startup the backend applies, in order, every migration not yet listed in `schema_migrations`, each in its own transaction, under a PostgreSQL advisory lock. Applied migrations must not be edited; add a new file instead.

Databases created before `schema_migrations` existed are detected by the presence of the `engines` table and have migration 001 marked as applied.

## REST API

All endpoints are under `/api` and use JSON. Errors have the form `{"error": "..."}`.

| Method and path | Description |
| --- | --- |
| `GET /api/health` | Liveness check. |
| `GET /api/engines` | List engines. |
| `POST /api/engines` | Register an engine: `{"name", "version", "image", "uci_options"}`. 409 when the name and version exist. |
| `GET /api/position-sets` | List position sets. |
| `POST /api/position-sets` | Import positions: `{"name", "format": "fen" \| "pgn", "content"}`. Returns 200 with the existing set when the same positions were imported before. |
| `GET /api/tournaments` | List tournaments. |
| `POST /api/tournaments` | Create and queue a tournament (body below). |
| `GET /api/tournaments/{id}` | Tournament with `progress` (games by status) and `score` (wins, draws, points). |
| `POST /api/tournaments/{id}/cancel` | Cancel a tournament. 409 when it already finished. |
| `GET /api/tournaments/{id}/games` | Games in schedule order. |
| `GET /api/games/{id}` | Game with moves, per-move telemetry and PGN. |
| `GET /api/games/{id}/pgn` | PGN file of a finished game. |

Tournament body:

```json
{
  "name": "v2 vs v1",
  "engine_a_id": "<uuid>",
  "engine_b_id": "<uuid>",
  "position_set_id": "<uuid>",
  "game_count": 100,
  "limit": { "movetime_ms": 1000 },
  "color_order": "alternate",
  "cpus": 1.0,
  "memory_mb": 256
}
```

`limit` takes exactly one of `movetime_ms`, `depth` or `nodes`. `game_count` defaults to 1000, `color_order` to `alternate` (`engine_a_white` and `engine_b_white` are also accepted).

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | HTTP listen address. |
| `DATABASE_URL` | — | PostgreSQL URL. Without it only `/ws` runs. |
| `CORS_ORIGINS` | `http://localhost:3000` | Browser origins allowed to use the API and `/ws`. |
| `ARENA_WORKERS` | `1` | Games played in parallel by this instance; `0` disables workers. |
| `ARENA_WORKER_ID` | hostname | Prefix of lease owner names; must differ between instances. |
| `ARENA_MOVE_MARGIN_MS` | `1000` | Tolerance on top of `movetime` before a time forfeit. |
| `ARENA_SEARCH_TIMEOUT_MS` | `60000` | Per-move deadline for `depth` and `nodes` limits. |
| `ARENA_ENGINE_NETWORK` | `none` | Docker network for engine containers. |
| `CHESS_ENGINE_VERSION`, `CHESS_ENGINE_IMAGES`, `CHESS_ENGINE_PATH` | | Engines offered on `/ws`; see [index-architecture.md](index-architecture.md). |

## Testing

```powershell
cd backend
go test ./...
```

Store and migration tests need PostgreSQL and are skipped unless `TEST_DATABASE_URL` is set. Each test runs in its own schema that is dropped afterwards:

```powershell
docker run -d --rm --name arena-test-pg -e POSTGRES_USER=arena -e POSTGRES_PASSWORD=arena -e POSTGRES_DB=arena_test -p 55432:5432 postgres:16-alpine
$env:TEST_DATABASE_URL = "postgres://arena:arena@localhost:55432/arena_test?sslmode=disable"
go test ./...
```

CI runs the same tests against a PostgreSQL service container.

## Known Limitations

- Engines are started through the Docker CLI rather than the Docker SDK for Go.
- Only per-move limits exist; there is no game clock (`wtime`/`btime`/`winc`/`binc`) yet.
- The `/ws` engine list comes from environment variables, separate from the `engines` table.
- The backend container runs as root to reach the Docker socket.
