# Chess Engines Testing App — Project Skeleton

This repository contains a minimal skeleton for the proposed system divided into three folders:

- `frontend/` — Next.js frontend with a chessboard and WebSocket demo.
- `backend/` — Go-based Arena WebSocket server (streams demo moves).
- `engines/` — Rust engine stub implementing a tiny UCI responder.

Quick start

1) Run the backend Arena (requires Go 1.20+):

```powershell
cd backend
go mod download
go run .
```

2) Run the frontend (requires Node.js 18+):

```powershell
cd frontend
npm install
npm run dev
```

3) Engines: compile or run the Rust stub (requires Rust toolchain):

```powershell
cd engines
cargo run
```

Notes

- The frontend connects to `ws://localhost:8080/ws` and will render moves sent by the backend.
- This skeleton is a starting point — extend the Arena to orchestrate Docker containers and implement full UCI engine features in `engines/`.
