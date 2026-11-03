# Chess Engines Testing App

This repository contains a minimal skeleton for the proposed system divided into three folders:

- `frontend/` — Next.js frontend: play against an engine (legal moves are highlighted when a piece is selected), create tournaments between any two registered engine versions, follow them live and replay games with engine telemetry.
- `backend/` — Go Arena: tournament REST API, workers that play engine-vs-engine games in Docker containers, and a WebSocket for playing against an engine. See [docs/System-architecture/arena-backend.md](docs/System-architecture/arena-backend.md).
- `engines/` — Rust chess engine with legal move generation, FEN, perft, UCI, and random legal move selection.

## Run with Docker

The easiest way to run the whole application. It requires only Docker (Docker Desktop on Windows and macOS); Rust, Go and Node.js are not needed.

1) Build the engine image. The backend starts engines from Docker images, so rebuild it after every change in `engines/`:

```powershell
docker build -t chess-arena-engine:local engines
```

2) Start PostgreSQL, the backend and the frontend (this also builds the Stockfish image used for analysis):

```powershell
docker compose up -d --build
```

3) Open http://localhost:3000:

- **Play** — play White against the engine.
- **Tournaments** — register engine versions (name, version and Docker image, for example `chess-arena-engine:local`), import starting positions (or add the built-in suite of 50 balanced openings with **Add opening suite**) and start a tournament between any two versions. With alternating colors, use twice as many games as positions so every position is played with both colors. Tick **Stop early with SPRT** to stop as soon as it is clear whether engine A is stronger. Each tournament shows the Elo difference with its error, LOS and the SPRT, and each game can be replayed move by move with the engine's telemetry and Stockfish's verdict on every move.
- **Leaderboard** — a rating for every engine version across all tournaments, with average accuracy and centipawn loss from Stockfish's analysis.

Pages update live as moves are played (the header shows **Live**). Stockfish (`chess-arena-stockfish:local`) can also be registered as an opponent, for example with UCI options `UCI_LimitStrength=true, UCI_Elo=1500`.

To test a new engine version against the previous one, publish its image and run `./scripts/regress.ps1 -Candidate 1.3.0`. Every tournament can be downloaded as PGN, games CSV or moves CSV, and the leaderboard estimates absolute Elo once Stockfish anchors (`UCI_LimitStrength=true, UCI_Elo=N`) have played. See [Analysis and Statistics](docs/System-architecture/analysis-and-statistics.md).

The engine has a fixed benchmark, `cargo run --release -- bench`. Its node count is checked by CI against `engines/bench-signature.txt`; see [Engine implementation](docs/System-architecture/engine-implementation.md#bench).

| Service | Address | Description |
| --- | --- | --- |
| `frontend` | http://localhost:3000 | Next.js application |
| `backend` | http://localhost:8080 | REST API (`/api/...`), WebSocket (`/ws`) and arena workers |
| `postgres` | `localhost:5432` | Database (user `chess_arena`, password `chess_arena_dev`) |

Useful commands:

```powershell
docker compose ps                  # service status
docker compose logs -f backend     # arena logs: games being played and finished
docker compose up -d --build       # rebuild after code changes
docker compose down                # stop; the database is kept
docker compose down -v             # stop and delete the database
```

Settings can be changed with environment variables or a `.env` file next to `docker-compose.yml`:

| Variable | Default | Meaning |
| --- | --- | --- |
| `ARENA_WORKERS` | `2` | Games played in parallel |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Backend address used by the browser; requires `--build` after a change |
| `CORS_ORIGINS` | `http://localhost:3000` | Frontend addresses allowed to call the backend |
| `CHESS_ENGINE_IMAGES` | empty | Extra `version=image` engines offered on the Play page, besides the engines registered on the Tournaments page |
| `CHESS_ENGINE_VERSION` | first engine | Engine selected when the Play page opens, e.g. `Alpha-Beta 1.2.0` |
| `ARENA_API_TOKEN` | empty | When set, creating engines, position sets and tournaments requires this token (the frontend asks for it) |
| `ARENA_ALLOWED_IMAGES` | `zienek/chess-arena-engine,chess-arena-engine,chess-arena-stockfish` | Docker image repositories engines may use (`namespace/*` allows a whole namespace) |
| `ANALYSIS_WORKERS` | `1` | Games analyzed by Stockfish in parallel; `0` disables analysis |
| `ANALYSIS_NODES` | `200000` | Stockfish nodes per analyzed position (about 1 s and depth 18 on one core) |

Notes:

- The backend starts engine containers through the host's Docker socket (`/var/run/docker.sock`), so every engine image must be available to the host's Docker or pullable from a registry.
- On Docker Desktop for Windows, starting a container takes several seconds and adds latency to every move. Use at least 1000 ms per move in tournaments, or engines may lose on time through no fault of their own.
- Port 3000, 8080 or 5432 must be free; stop a local `npm run dev` or PostgreSQL first.

## Local development

1) Build the Rust engine (requires Rust toolchain):

```powershell
cd engines
cargo build
```

2) Run the backend Arena (requires Go 1.25+):

```powershell
cd backend
go mod download
go run .
```

Without `DATABASE_URL` only the WebSocket game runs. With it (for example `docker compose up postgres` and `DATABASE_URL=postgres://chess_arena:chess_arena_dev@localhost:5432/chess_arena?sslmode=disable`) the tournament API and arena workers start as well.

The backend expects the engine at `../engines/target/debug/chess-arena-engine.exe` on Windows or `../engines/target/debug/chess-arena-engine` on other systems. Set `CHESS_ENGINE_PATH` to override this location.

3) Run the frontend (requires Node.js 20+):

```powershell
cd frontend
npm install
npm run dev
```

### Notes

- The frontend talks to the backend at `http://localhost:8080` (REST and `ws://…/ws`). Set `NEXT_PUBLIC_API_URL` to use another address.
- Tournaments (`/tournaments`) need the backend with a database, for example `docker compose up -d`.
- The player controls White and sends moves through WebSocket.
- The backend validates player moves with its own chess rules and asks the engine for a move.
- The frontend lists configured engine versions and starts a new game with the selected version.
- The backend sends available versions in the initial `state` message and accepts the selected version in a `reset` message.
- The current engine does not search for the best move yet.
