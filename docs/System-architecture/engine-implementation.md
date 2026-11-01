# Chess Engine Implementation

## Scope

The Rust engine currently provides a functional chess rules core and a basic UCI process interface. It does not implement a search algorithm or a positional evaluation yet.

The current implementation includes:

- 64-bit bitboard representation using `u64`.
- Piece, color, and square enumerations.
- Board state management.
- Move execution and undo.
- Attack generation.
- Pseudo-legal and legal move generation.
- FEN parsing and serialization.
- Deterministic position hashing and threefold-repetition tracking.
- Perft validation.
- Basic game-status detection.
- Basic UCI command handling.
- Random selection of a legal move for `go`.

## Source Layout

The engine is located in `engines/`.

| File | Responsibility |
| --- | --- |
| `src/main.rs` | Reads UCI commands from standard input and writes responses to standard output. |
| `src/board.rs` | Bitboards, board state, piece placement, move execution, and undo. |
| `src/moves.rs` | Move types, move categories, undo state, and UCI move serialization. |
| `src/attacks.rs` | Pawn, knight, king, bishop, rook, and queen attack generation. |
| `src/movegen.rs` | Pseudo-legal moves, legal move filtering, check detection, and castling validation. |
| `src/fen.rs` | FEN parsing, validation, and board serialization. |
| `src/hash.rs` | Deterministic position hashing for repetition detection. |
| `src/perft.rs` | Recursive legal move counting and divide support. |
| `src/status.rs` | Checkmate, stalemate, fifty-move, insufficient-material, and ongoing status detection. |
| `src/uci.rs` | UCI command handling; runs the search on its own thread and streams its output. |
| `src/eval.rs` | Static evaluation: material and piece-square tables. |
| `src/search/mod.rs` | Iterative deepening and `SearchContext`: node and seldepth counters, time and node limits, `stop`, iteration reports. |
| `src/search/alphabeta.rs` | Negamax with alpha-beta pruning, mate scores, draw detection, principal variation. |
| `src/search/ordering.rs` | Move ordering: previous best move, then MVV-LVA captures, then promotions. |
| `src/search/limits.rs` | Parsing of `go` arguments and the time budget per move. |
| `src/search/report.rs` | `Score` (centipawns or mate) and the `info` line of an iteration. |
| `Cargo.toml` | Rust package metadata; the engine has no dependencies. |

## Board Representation

The board uses a little-endian rank-file mapping:

- `A1` is bit `0`.
- `H1` is bit `7`.
- `A8` is bit `56`.
- `H8` is bit `63`.

`Bitboard` wraps a `u64` and exposes the basic operations required by the engine:

- `set_bit(square)`
- `clear_bit(square)`
- `test_bit(square)`
- `count_bits()` using `count_ones()`
- `lsb_index()` using `trailing_zeros()`
- `pop_lsb()` for iterating through set bits

`Board` stores two complementary groups of bitboards:

- One bitboard for each `PieceKind`.
- One bitboard for each `Color`.

It also stores the complete game state:

- Side to move.
- Castling rights.
- En passant target square.
- Halfmove clock.
- Fullmove number.
- A square lookup cache used to identify the piece and color on a square.

The square cache is updated together with the bitboards by `set_piece`, `clear_square`, and move operations.

## Move Model

`Move` contains:

- Source square.
- Destination square.
- Optional promotion piece.
- `MoveKind`.

Supported move kinds are:

- Quiet move.
- Capture.
- Double pawn push.
- King-side castling.
- Queen-side castling.
- En passant.
- Promotion.
- Promotion capture.

`Undo` stores the state needed to restore the position after a move:

- Captured piece.
- Previous castling rights.
- Previous en passant square.
- Previous halfmove clock.
- Previous fullmove number.

`Board::make_move` updates all bitboards, the square cache, counters, castling rights, en passant state, and side to move. `Board::unmake_move` restores the previous position.

## Attack Generation

`attacks.rs` currently uses a simple ray-tracing implementation for sliding pieces. This approach is intentionally straightforward and serves as the correctness baseline before future optimizations.

Implemented attacks:

- White and black pawn attacks.
- Knight attacks.
- King attacks.
- Bishop rays.
- Rook rays.
- Queen rays.
- Board-aware `is_square_attacked`.

Sliding attacks include the first occupied square and stop immediately after it. This allows the first occupied square to be captured while preventing attacks beyond a blocker.

## Move Generation

The move generator works in two stages.

### Pseudo-legal moves

`pseudo_legal_moves` generates moves according to piece movement rules. It handles:

- Normal moves for all pieces.
- Captures.
- Pawn single and double pushes.
- Promotions to queen, rook, bishop, or knight.
- En passant.
- Castling when the required squares are empty and the rook is present.

### Legal moves

`legal_moves` filters pseudo-legal moves by:

1. Executing each move.
2. Locating the moving side's king.
3. Checking whether the king is attacked.
4. Undoing the move.
5. Keeping the move only when the king is safe.

Castling has an additional check for the starting, transit, and destination squares. A castle is rejected if any of those squares is attacked by the opponent.

This implementation prioritizes correctness and readability. It does not yet use pinned-piece masks, check evasion masks, or specialized legal move generation.

## FEN Support

`fen.rs` supports all six FEN fields:

1. Piece placement.
2. Side to move.
3. Castling rights.
4. En passant target square.
5. Halfmove clock.
6. Fullmove number.

`parse_fen` returns a `FenError` instead of panicking for malformed input. `board_to_fen` serializes the complete board state back to FEN.

FEN round-trip tests currently cover the standard starting position and positions containing en passant, castling rights, counters, and mixed pieces.

## Perft Validation

`perft` recursively counts legal positions without evaluation or search heuristics. It is the primary correctness check for the move generator and make/unmake implementation.

The test suite checks six standard positions from [Chess Programming Wiki](https://www.chessprogramming.org/Perft_Results), which together cover castling, en passant, promotions, discovered checks, and pins:

| Position | Fast tests (every `cargo test`) | Deep test (`--ignored`) |
| --- | --- | ---: |
| Start position | depths 1–4 | depth 5: 4,865,609 |
| Kiwipete | depths 1–3 | depth 5: 193,690,690 |
| Position 3 | depths 1–4 | depth 6: 11,030,083 |
| Position 4 | depths 1–3 | depth 5: 15,833,292 |
| Position 5 | depths 1–3 | depth 4: 2,103,487 |
| Position 6 | depths 1–3 | depth 4: 3,894,594 |

The deep tests are marked `#[ignore]` because they take about a minute in release mode, mostly Kiwipete. CI runs them as a separate step:

```powershell
cargo test --release perft -- --ignored
```

`divide` also provides a per-root-move breakdown for debugging move generation.

## Game Status

`status.rs` currently detects:

- Ongoing positions.
- Checkmate.
- Stalemate.
- The fifty-move draw threshold.
- Basic insufficient material such as king versus king and king plus one minor piece versus king.

Status detection first checks for checkmate or stalemate, then applies draw conditions. The UCI engine stores the position hash after the initial position and after every applied move, allowing threefold repetition to be detected.

The hash includes piece placement, side to move, castling rights, and relevant en passant state. Halfmove and fullmove counters are intentionally excluded because they do not define positional identity for repetition.

## UCI Interface

The executable reads one command per line from standard input and writes responses to standard output.

Currently handled commands:

- `uci`
- `isready`
- `ucinewgame`
- `position startpos`
- `position startpos moves ...`
- `position fen <six FEN fields>`
- `position fen <six FEN fields> moves ...`
- `go` with `movetime`, `depth`, `nodes`, `wtime`, `btime`, `winc`, `binc`, `movestogo`, `infinite` and `ponder`
- `stop`
- `d`
- `quit`

Invalid commands and invalid positions are reported as `info string error <message>`. The backend treats only lines with this prefix as errors; any other `info string` line is plain logging, as the UCI protocol allows, and is ignored.

### Searching

`go` starts the search on a separate thread, so the engine keeps reading commands while it thinks:

- `isready` is answered at once, even during a search.
- `stop` ends the search; the engine then sends `bestmove`.
- `position`, `ucinewgame`, `go` and `quit` stop a running search first.
- With `go infinite` (or `go ponder`), `bestmove` is sent only after `stop`, as the UCI protocol requires.
- Unsupported or malformed arguments, such as `go searchmoves`, are rejected with `info string error ...` and no search starts.

The time budget for one move is:

| `go` arguments | Budget |
| --- | --- |
| `movetime T` | `T` |
| `wtime`/`btime` with optional `winc`/`binc` and `movestogo` | `time / movestogo + 3/4 × increment` (30 moves assumed without `movestogo`), at most the clock minus 50 ms |
| only `depth` or `nodes` | none; the search ends at the depth or node limit |
| nothing (bare `go`) | none; the search stops at depth 5 |
| `infinite` | none; the search ends on `stop` |

### Evaluation

`eval.rs` scores a position in centipawns from the side to move:

- **Material:** pawn 100, knight 320, bishop 330, rook 500, queen 900.
- **Piece-square tables:** a bonus or penalty for every piece on every square, from Tomasz Michniewski's [Simplified Evaluation Function](https://www.chessprogramming.org/Simplified_Evaluation_Function). Knights prefer the center, pawns gain value as they advance, rooks like the seventh rank.
- **King:** blended between a middlegame table (stay sheltered behind pawns) and an endgame table (walk to the center), weighted by the knights, bishops, rooks and queens left on the board.

### Search

`search()` runs **iterative deepening**: it searches to depth 1, then 2, 3 and so on, and sends an `info` line after each completed depth. Each depth is a **negamax search with alpha-beta pruning** (`alphabeta.rs`):

- Leaves (depth 0) are scored by `evaluate`. There is no quiescence search yet.
- Checkmate scores `-(MATE - ply)`, so the search prefers the fastest mate and the slowest defeat. Scores near `MATE` are reported as `score mate N`.
- Stalemate, the fifty-move rule, insufficient material and repetitions score 0. A position that already occurred in the game or on the search path counts as a draw, so the engine both avoids repetitions when winning and seeks them when losing; for this the UCI layer passes the game's position history to the search.
- The principal variation is collected while searching and reported as `pv`.

**Move ordering** (`ordering.rs`) makes the pruning effective: the best move of the previous depth (and the rest of its line) is searched first, then captures ordered by MVV-LVA (most valuable victim, least valuable attacker), then promotions, then quiet moves.

**Stopping:**

- Depth 1 always completes, so a legal move is always returned, even with `nodes 1` or `movetime 1`.
- After that, the clock and the stop flag are checked every 1024 nodes, and the node limit at every node. An interrupted depth is discarded; the best move of the last completed depth is played.
- No new depth starts after half of the time budget, because it would rarely finish.
- The search ends early when it finds a forced mate within its depth, or when there is only one legal move.

Every algorithm reports through `SearchContext`: `visit(ply)` for every position entered, `poll_stop()` to check the limits, and `report(depth, score, pv)` after every completed depth. The context turns reports into `info` lines and keeps the counters for the telemetry.

### Search telemetry

After every completed iteration the engine sends a standard UCI `info` line, and finally `bestmove`. The arena keeps the last `info` line before `bestmove` and stores every field in `game_moves`.

```text
info depth 1 seldepth 1 score cp 0 nodes 20 nps 9640 time 2 pv e2e4
bestmove e2e4
```

| Field | Meaning | Database column |
| --- | --- | --- |
| `depth` | Completed iteration depth in plies | `depth` |
| `seldepth` | Deepest ply reached, including extensions | `seldepth` |
| `score cp <n>` | Evaluation in centipawns from the side to move | `score_cp` |
| `score mate <n>` | Mate in `n` moves; negative when the side to move gets mated | `score_mate` |
| `nodes` | Positions visited since the search started | `nodes` |
| `nps` | Nodes per second | `nps` |
| `time` | Milliseconds since the search started | `engine_time_ms` |
| `pv` | Principal variation in UCI notation | `pv` |

The arena also stores its own measurement of the time between `go` and `bestmove` as `think_time_ms`.

When there is no legal move, the engine reports depth 0 and no `pv`: `info depth 0 seldepth 0 score mate 0 nodes 0 nps 0 time 0` for checkmate, or `score cp 0` for stalemate, followed by `bestmove 0000`.

## Running the Engine

From the repository root:

```powershell
Set-Location engines
cargo run
```

A simple UCI session can be sent through standard input:

```powershell
@('uci', 'isready', 'position startpos moves e2e4', 'go', 'quit') | cargo run --quiet
```

With `go movetime 1000`, the release build typically reaches depth 5 from the starting position (about 100,000 nodes).

## Verification

The current crate passes:

```powershell
cargo test
cargo clippy --all-targets --all-features -- -D warnings
```

The test suite covers:

- Bitboard operations.
- Move execution and undo for special moves.
- Attack masks and blockers.
- Check and castling legality.
- FEN parsing and serialization.
- Perft on six reference positions (shallow by default, deep with `--ignored`).
- Evaluation, move ordering and alpha-beta search: mates in one and two, winning material, avoiding stalemate and repetition draws, limits and `stop`.
- UCI command handling and `info` telemetry.

## Versioning and Releases

An engine version is defined in one place, `engines/Cargo.toml`, and appears identically everywhere else:

| Where | Form | Example |
| --- | --- | --- |
| `engines/Cargo.toml` | `X.Y.Z` | `version = "1.1.0"` |
| Git tag | `engine-vX.Y.Z` | `engine-v1.1.0` |
| Docker image | `<user>/chess-arena-engine:X.Y.Z` | `zienek/chess-arena-engine:1.1.0` |
| UCI handshake | `id name chess-arena-engine X.Y.Z` | `id name chess-arena-engine 1.1.0` |

Versions follow [semantic versioning](https://semver.org): raise the major version when playing strength or behavior changes incompatibly, the minor version for new features such as a better evaluation, and the patch version for fixes. Prereleases use a suffix, such as `1.2.0-rc.1`.

### Publishing a version

1. Set the new version in `engines/Cargo.toml` and run `cargo build` so `Cargo.lock` is updated.
2. Commit, then tag the commit and push the tag:

   ```powershell
   git tag engine-v1.1.0
   git push origin engine-v1.1.0
   ```

The `Publish engine image` workflow then:

1. rejects the tag unless it has the form `engine-vX.Y.Z` (optionally with a prerelease suffix) and matches `engines/Cargo.toml`;
2. runs the full CI on the tagged commit;
3. builds the image from the tagged commit and pushes these tags:

| Image tag | Pushed when |
| --- | --- |
| `X.Y.Z` | always |
| `X.Y` | the version is the newest stable patch of `X.Y` |
| `latest` | the version is the newest stable version overall |

Publishing an older version again never moves `X.Y` or `latest` back to it, and prereleases never move them.

To publish an existing tag again, for example after a failed run, start the workflow manually under **Actions → Publish engine image → Run workflow** and enter the tag, such as `engine-v1.1.0`. The workflow builds that tag, whichever branch it is started from.

In tournaments, register engines by their exact version tag (`zienek/chess-arena-engine:1.1.0`) rather than `latest` or `X.Y`. These two tags move to newer versions, so results would no longer show which version played. The arena pulls an image the first time a game needs it.

The image `chess-arena-engine:engine-v1.0` was published before this scheme existed. It predates search telemetry and the `info string error` prefix.

## Current Limitations

The following features are not implemented yet:

- Quiescence search; without it the evaluation swings between odd and even depths (horizon effect).
- Transposition tables.
- Killer moves and history heuristics for ordering quiet moves.
- Evaluation terms beyond material and piece-square tables (pawn structure, mobility, king safety).
- A broader repetition-history API outside the UCI engine.
- Optimized magic bitboards or generated lookup tables.
- Library crate exports; the current project is primarily a binary crate.

## Recommended Next Steps

1. Add quiescence search over captures.
2. Add a transposition table keyed by the position hash.
3. Add killer moves and history heuristics.
4. Introduce benchmarks before optimizing slider attacks or board mutation.
