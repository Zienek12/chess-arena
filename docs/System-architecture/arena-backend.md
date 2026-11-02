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
| `analysis` | Reviews finished games with Stockfish and judges every move. |
| `stats` | Elo, error bars, LOS, SPRT and leaderboard ratings. |
| `events` | Delivers PostgreSQL `NOTIFY` change events to `/api/events` subscribers. |
| `api` | REST API and event stream for the frontend. |
| `play` | Human-versus-engine WebSocket at `/ws`, used by the current frontend. |

```text
main ──► config, database, api, arena, analysis, events, play
api ──► store, tournament, sandbox, events
arena ──► store, sandbox, uci, rules, tournament
analysis ──► store, sandbox, uci, rules
events ──► store
store ──► tournament, stats, uci
play ──► store, sandbox, uci, rules
tournament ──► rules, stats
```

`rules`, `uci` and `sandbox` do not depend on the database, so they can be tested and reused on their own.

## How a Tournament Runs

1. A client registers engines (`POST /api/engines`) and imports a position set (`POST /api/position-sets`).
2. `POST /api/tournaments` validates the configuration and, in one transaction, stores the tournament as `queued` and schedules every game. With `alternate` colors, games 1–2 share a position with colors swapped, games 3–4 share the next one, and so on. By default (`position_order: random`) the positions are shuffled with a seed derived from the tournament ID: the schedule is reproducible, and a short tournament, or one stopped early by an SPRT, samples the whole set instead of only its first positions. `sequential` uses the set's order.
3. Each worker claims the next game with `SELECT … FOR UPDATE SKIP LOCKED`, so several workers or backend instances never play the same game. The claim is a lease that expires after 5 minutes unless renewed.
4. The worker starts both engines as containers, performs the UCI handshake and plays the game:
   - every move must arrive within `movetime + ARENA_MOVE_MARGIN_MS`, or within `ARENA_SEARCH_TIMEOUT_MS` for `depth` and `nodes` limits;
   - every move is validated by `rules`; checkmate, stalemate, threefold repetition, the fifty-move rule and insufficient material end the game;
   - each move is stored with its telemetry in the same transaction that renews the lease.
5. When the game ends, the result, termination and PGN are stored. With an SPRT, the test is evaluated in the same transaction and, once it decides, the tournament completes and its remaining games are cancelled. Otherwise the tournament becomes `completed` after its last game.
6. An analysis worker then reviews the game with Stockfish; see [Analysis and Statistics](analysis-and-statistics.md).

Every change (claim, move, result, analysis, tournament status) is announced with `pg_notify('arena_events', …)` inside its transaction, so it is delivered only when committed. Each backend instance listens on that channel and forwards the events to the browsers connected to `/api/events`.

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

## Security

The backend controls the host's Docker daemon, so it only starts trusted images and, optionally, only accepts changes from holders of a token.

- **Image allowlist.** `ARENA_ALLOWED_IMAGES` lists the image repositories engines may use: exact repositories (`zienek/chess-arena-engine`), namespaces (`zienek/*`) or `*` for any image. Tags and digests are ignored. It is checked when an engine is registered (422 otherwise) and again by the Docker launcher before every container start, which also covers engines stored earlier and the Play page's configured images.
- **API token.** When `ARENA_API_TOKEN` is set, every `POST` requires `Authorization: Bearer <token>` (401 otherwise); reads stay public. `GET /api/auth` tells the frontend whether a token is required and whether the one it sent is valid. The frontend asks for the token and keeps it in the browser's local storage. Without a token the backend logs a warning at startup.

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
| `GET /api/tournaments/{id}` | Tournament with `progress` (games by status), `score` (wins, draws, points), `stats` (Elo ± error, LOS), `sprt_status` (LLR and bounds, when an SPRT is configured) and `telemetry` (per engine: moves, average and max depth, average and total nodes, average NPS and think time, and the move quality from analysis). |
| `POST /api/tournaments/{id}/cancel` | Cancel a tournament. 409 when it already finished. |
| `GET /api/tournaments/{id}/games` | Games in schedule order, each with its starting position's number in the set and opening name (`position_number`, `opening`). |
| `GET /api/games/{id}` | Game with moves, per-move telemetry and analysis, the analysis status, and PGN. |
| `GET /api/games/{id}/pgn` | PGN file of a finished game. |
| `GET /api/leaderboard` | Every engine with finished games: rating, results, average depth and NPS, and move quality. |
| `GET /api/auth` | Whether writes need the API token, and whether the request's token is valid. |
| `GET /api/events` | Server-sent events `{"type", "tournament_id", "game_id"}` for every change; `?tournament_id=` and `?game_id=` narrow the stream. `resync` asks clients to reload everything. |

Every `POST` needs the API token when `ARENA_API_TOKEN` is set.

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
  "position_order": "random",
  "cpus": 1.0,
  "memory_mb": 256,
  "sprt": { "elo0": 0, "elo1": 10, "alpha": 0.05, "beta": 0.05 }
}
```

`limit` takes exactly one of `movetime_ms`, `depth` or `nodes`. `game_count` defaults to 1000, `color_order` to `alternate` (`engine_a_white` and `engine_b_white` are also accepted). `position_order` is `random` (default) or `sequential`. `sprt` is optional; `alpha` and `beta` default to 0.05.

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
| `ARENA_ALLOWED_IMAGES` | `zienek/chess-arena-engine,chess-arena-engine,chess-arena-stockfish` | Image repositories engines may use; see [Security](#security). |
| `ARENA_API_TOKEN` | — | Token required for every `POST`; unset means no authentication. |
| `ANALYSIS_WORKERS` | `1` | Games analyzed in parallel; `0` disables analysis. |
| `ANALYSIS_ENGINE_IMAGE` | `chess-arena-stockfish:local` | Docker image of the analysis engine. |
| `ANALYSIS_NODES` | `200000` | Search budget per analyzed position. |
| `CHESS_ENGINE_VERSION`, `CHESS_ENGINE_IMAGES`, `CHESS_ENGINE_PATH` | | Engines offered on `/ws` besides the registered ones; see [index-architecture.md](index-architecture.md). |

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
- The backend container runs as root to reach the Docker socket.
