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
| `src/uci.rs` | UCI command handling and random legal move selection. |
| `Cargo.toml` | Rust package metadata and the `rand` dependency. |

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

The starting position currently matches these reference values:

| Depth | Nodes |
| ---: | ---: |
| 1 | 20 |
| 2 | 400 |
| 3 | 8902 |

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
- `go`
- `stop`
- `d`
- `quit`

Invalid commands and invalid positions are reported as `info string error <message>`. The backend treats only lines with this prefix as errors; any other `info string` line is plain logging, as the UCI protocol allows, and is ignored.

### Current `go` behavior

The engine does not search the game tree yet. For `go` it:

1. Generates all legal moves for the current position.
2. Selects one move uniformly from the generated vector using `rand` and `thread_rng`.
3. Returns the move in UCI notation.
4. Returns `bestmove 0000` if there are no legal moves.

The random choice is intentionally temporary. It provides a valid UCI response while leaving the rules core independent from future search code.

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

The selected move after `go` is expected to vary because it is random, but it must always be legal for the current position.

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
- Starting-position perft at depths one through three.
- UCI command handling and legal random move selection.

## Current Limitations

The following features are not implemented yet:

- Minimax, alpha-beta, or any other search algorithm.
- Quiescence search.
- Transposition tables.
- Static position evaluation.
- A broader repetition-history API outside the UCI engine.
- Full UCI time-control handling such as `wtime`, `btime`, and `movetime`.
- Optimized magic bitboards or generated lookup tables.
- Library crate exports; the current project is primarily a binary crate.

## Recommended Next Steps

1. Add explicit game-status detection for checkmate, stalemate, the fifty-move rule, repetition, and insufficient material.
2. Add perft positions covering castling, en passant, promotions, and tactical checks.
3. Add a static evaluator.
4. Replace random `go` behavior with a small deterministic search, then add alpha-beta pruning.
5. Introduce benchmarks before optimizing slider attacks or board mutation.
6. Add time-control parsing and cancellation support to the UCI layer.
