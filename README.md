# Chess Engines Testing App

This repository contains a minimal skeleton for the proposed system divided into three folders:

- `frontend/` — Next.js frontend with a chessboard and WebSocket demo.
- `backend/` — Go-based Arena WebSocket server that manages a Rust UCI engine process.
- `engines/` — Rust chess engine with legal move generation, FEN, perft, UCI, and random legal move selection.

Quick start

1) Build the Rust engine (requires Rust toolchain):

```powershell
cd engines
cargo build
```

2) Run the backend Arena (requires Go 1.20+):

```powershell
cd backend
go mod download
go run .
```

The backend expects the engine at `../engines/target/debug/uci_stub.exe` on Windows or `../engines/target/debug/uci_stub` on other systems. Set `CHESS_ENGINE_PATH` to override this location.

3) Run the frontend (requires Node.js 18+):

```powershell
cd frontend
npm install
npm run dev
```

Notes

- The frontend connects to `ws://localhost:8080/ws`.
- The player controls White and sends moves through WebSocket.
- The backend validates player moves through the Rust engine and requests a random legal engine move.
- The frontend lists configured engine versions and starts a new game with the selected version.
- The backend sends available versions in the initial `state` message and accepts the selected version in a `reset` message.
- The current engine does not search for the best move yet.
