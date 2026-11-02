# Engine Internals (version 1.2.0)

This document explains how the Rust engine in `engines/` turns a `go` command into a move: how it represents the board, generates and validates moves, evaluates positions and searches the game tree. It describes the code as it is in version 1.2.0, including its known weaknesses, so that every later improvement can be measured against it.

For the module list, UCI commands, building and releasing, see [Engine implementation](engine-implementation.md).

## Contents

1. [From `go` to `bestmove`](#1-from-go-to-bestmove)
2. [Board representation](#2-board-representation)
3. [Moves and make/unmake](#3-moves-and-makeunmake)
4. [Attack generation](#4-attack-generation)
5. [Move generation](#5-move-generation)
6. [Position hashing](#6-position-hashing)
7. [Evaluation](#7-evaluation)
8. [Search](#8-search)
9. [Time management and stopping](#9-time-management-and-stopping)
10. [Telemetry](#10-telemetry)
11. [A search, step by step](#11-a-search-step-by-step)
12. [Known weaknesses](#12-known-weaknesses)

## 1. From `go` to `bestmove`

```text
stdin ──► main.rs: one line at a time ──► uci.rs: UciEngine::handle_command
                                               │
        "position startpos moves e2e4 …"       │  replays the moves on a Board and records
                                               │  the hash of every position (history)
        "go movetime 1000"                     │  parses SearchLimits, copies the Board,
                                               ▼  spawns a search thread
                                     search::search (iterative deepening)
                                               │  depth 1, 2, 3, …
                                               ▼
                                     search_root ──► Search::negamax (alpha-beta, recursive)
                                               │        ├─ legal_moves (movegen.rs)
                                               │        ├─ order_moves (ordering.rs)
                                               │        ├─ make_move / unmake_move (board.rs)
                                               │        ├─ is_draw (repetition, 50 moves, material)
                                               │        └─ evaluate (eval.rs) at the leaves
                                               ▼
                    after each completed depth: "info depth … score … nodes … pv …"
                    at the end:                 "bestmove <move>"
```

The command loop stays responsive while the search runs on its own thread: `isready` is answered immediately, and `stop`, `position`, `ucinewgame`, `go` and `quit` set the search's stop flag and wait for its `bestmove`.

## 2. Board representation

`board.rs` keeps every position in two redundant forms that are always updated together:

| Structure | Content | Used for |
| --- | --- | --- |
| `piece_bitboards: [Bitboard; 6]` | One 64-bit mask per piece kind (pawn … king), both colors | Counting pieces, iterating pieces in evaluation |
| `color_bitboards: [Bitboard; 2]` | One mask per color | Own/enemy occupancy, blocking sliding pieces |
| `squares: [Option<(PieceKind, Color)>; 64]` | A "mailbox": what stands on each square | Constant-time `piece_at(square)` |

Plus the game state: `side_to_move`, `castling_rights` (4 bits: white king side, white queen side, black king side, black queen side), `en_passant` (the square behind a pawn that just moved two squares), `halfmove_clock` (for the fifty-move rule) and `fullmove_number`.

Squares are numbered "little-endian rank-file": `a1 = 0`, `b1 = 1`, …, `h1 = 7`, `a2 = 8`, …, `h8 = 63`. So `file = index % 8` and `rank = index / 8`, and bit `n` of a bitboard is square `n`.

`set_piece` and `clear_square` are the only places that change pieces; each updates the mailbox and both bitboards, so they cannot get out of sync. The whole `Board` is a small `Copy` value. The UCI layer copies it into the search thread instead of sharing it.

## 3. Moves and make/unmake

A `Move` (`moves.rs`) holds `from`, `to`, an optional promotion piece and a `MoveKind`:

| `MoveKind` | Meaning |
| --- | --- |
| `Quiet` | Ordinary move to an empty square |
| `Capture` | Takes the piece on `to` |
| `DoublePawnPush` | Pawn moves two squares and creates an en passant square |
| `KingCastle`, `QueenCastle` | Castling; the king's move is stored, the rook moves implicitly |
| `EnPassant` | Pawn takes the pawn beside it, landing behind it |
| `Promotion`, `PromotionCapture` | Pawn reaches the last rank (with or without a capture) |

The kind is decided when the move is generated, so `make_move` never has to rediscover what kind of move it is. In UCI a move is written as `from` + `to` + an optional promotion letter, for example `e2e4`, `e1g1` (castling) or `e7e8q`.

**`make_move`** applies a move in place and returns an `Undo` record with everything that cannot be recomputed: the captured piece, the previous castling rights, en passant square, halfmove clock and fullmove number. It:

1. clears `from` and `to` and puts the piece (or the promoted piece) on `to`;
2. handles the special kinds: removes the pawn taken en passant, moves the rook when castling, sets the en passant square after a double push;
3. removes castling rights when the king moves, when a rook leaves its corner, or when a rook is captured in its corner;
4. resets the halfmove clock after a pawn move or capture, otherwise increments it;
5. increments the fullmove number after Black's move and switches the side to move.

**`unmake_move(move, undo)`** reverses these steps exactly. Searching therefore never copies boards: it walks down the tree with `make_move` and back up with `unmake_move`. Tests check that make followed by unmake restores the identical board for every move kind.

## 4. Attack generation

`attacks.rs` computes which squares a piece attacks, directly from coordinates, every time it is asked:

- **Pawn, knight, king**: a fixed list of (file, rank) offsets, keeping only the ones that stay on the board. A pawn attacks the two squares diagonally forward.
- **Bishop, rook, queen** (sliding pieces): walk each direction one square at a time, adding squares until the edge or the first occupied square, which is included (it may be an enemy piece to capture). The queen combines the bishop's and the rook's rays.

`is_square_attacked(board, square, by_color)` loops over all 64 squares, and for every piece of `by_color` computes its attacks and checks whether they contain `square`.

This approach is simple and easy to verify, but it is the most expensive part of the engine (see [Known weaknesses](#12-known-weaknesses)). Faster engines precompute the attack tables of pawns, knights and kings, and use magic bitboards for sliding pieces.

## 5. Move generation

### Pseudo-legal moves

`pseudo_legal_moves` follows the rules of how pieces move but ignores whether the own king is left in check. For every piece of the side to move:

- **Knight, bishop, rook, queen, king**: the attack set minus squares occupied by own pieces. Each target becomes a `Capture` if an enemy piece stands there, otherwise `Quiet`.
- **Pawn**: one square forward if empty; two squares from the starting rank if both are empty (`DoublePawnPush`); diagonal captures of enemy pieces; en passant onto `board.en_passant`. A move to the last rank produces four moves, promoting to a queen, rook, bishop and knight.
- **Castling**: if the side still has the right, the king and rook stand on their original squares, and the squares between them are empty (f1, g1 for the king side; b1, c1, d1 for the queen side).

### Legal moves

`legal_moves` filters the pseudo-legal moves:

1. A castling move is rejected if the king's start square, the square it crosses or its destination is attacked (you cannot castle out of, through or into check).
2. Every other move is played with `make_move`; the move is legal if the own king is not attacked afterwards. The move is then taken back with `unmake_move`.

This "make, test, unmake" method handles every subtle case (pins, discovered checks, en passant that exposes the king along a rank, king moves next to the enemy king) without special code, at the price of one `make_move`, `unmake_move` and attack test per pseudo-legal move.

### Verification with perft

Perft counts all leaf positions of the legal move tree to a fixed depth and compares them with published reference numbers. `perft.rs` checks six standard positions. The start position gives 20, 400, 8,902, 197,281 and 4,865,609 positions at depths 1–5. Kiwipete, full of castling, en passant and promotion traps, gives 48, 2,039 and 97,862 at depths 1–3, and 193,690,690 at depth 5. Any bug in move generation changes these numbers. CI runs the fast perft tests on every change, and the deep ones run with `cargo test --release perft -- --ignored`.

## 6. Position hashing

`hash.rs` gives every position a 64-bit fingerprint, used to detect repeated positions. The hash is the XOR of one pseudo-random key per:

- piece on a square (6 kinds × 2 colors × 64 squares),
- side to move (when Black is to move),
- castling-rights combination,
- en passant file, but only if an en passant capture is actually possible. A double pawn push with no enemy pawn beside it does not make the position different.

The keys come from the SplitMix64 mixing function applied to a fixed number per feature, so no key table has to be stored. The halfmove clock and move number are not part of the hash, so the same position reached at different moves has the same hash.

The hash is computed from scratch, by scanning all 64 squares, for every position the search visits. Engines usually update it incrementally in `make_move` (XOR out the old square, XOR in the new one).

## 7. Evaluation

`eval.rs` scores a position statically, without looking at any moves, in centipawns (1/100 of a pawn) from the point of view of the side to move: positive means the side to move is better. The score is the sum, over all pieces, of a material value and a positional bonus.

### Material

| Piece | Pawn | Knight | Bishop | Rook | Queen | King |
| --- | --- | --- | --- | --- | --- | --- |
| Value | 100 | 320 | 330 | 500 | 900 | 0 |

The values follow Tomasz Michniewski's "Simplified Evaluation Function". The bishop is worth slightly more than the knight. The king has no material value because it is never captured.

### Piece-square tables

Each piece kind has a 64-entry table of bonuses for standing on each square. The tables are written from White's point of view with rank 8 at the top, so they read like a board diagram. For a White piece the square is mirrored vertically before the lookup, so both colors share the same tables. In words:

| Piece | What the table encourages |
| --- | --- |
| Pawn | Advancing, especially the central d- and e-pawns (+20 on d4/e4, −20 while they still stand on d2/e2); +50 on the seventh rank |
| Knight | The center (+20 on d4, e4, d5, e5); −50 in the corners ("a knight on the rim is dim") |
| Bishop | Long diagonals and the center; avoiding corners and edges |
| Rook | The seventh rank (+10); d1/e1 over the corners; mostly neutral |
| Queen | Mild centralization; avoiding edges |
| King | Depends on the game phase, below |

Example: after 1.Nc3 the knight moves from b1 (−40) to c3 (+10), so White gains 50. This is the `score cp 50` the engine reports at depth 1 from the start position.

### Tapered king evaluation

A king should hide behind its pawns while queens and rooks are on the board, and walk to the center in the endgame. There are two king tables, one for the middlegame (castled squares +20 to +30, the center −40 to −50) and one for the endgame (center +40, corners −50). They are blended by the game phase:

```text
phase      = min(256, non_pawn_material × 256 / 6400)    # 6400 = all knights, bishops, rooks and queens at the start
king_bonus = (middlegame[sq] × phase + endgame[sq] × (256 − phase)) / 256
```

With all pieces on the board the middlegame table counts fully. With only kings and pawns left, the endgame table does. In between the two are mixed in proportion to the material that remains.

### What the evaluation does not know

Everything beyond material and piece placement is invisible to it: pawn structure (doubled, isolated and passed pawns), mobility, king safety beyond the table (open files towards the king, attackers near it), the bishop pair, rooks on open files, threats and hanging pieces. A piece that is attacked and undefended still counts at full value until the search sees it being taken.

## 8. Search

Searching means looking ahead: playing candidate moves, the opponent's replies, and so on, then choosing the move that leads to the best position the side to move can force. The engine combines four techniques: negamax with alpha-beta pruning, iterative deepening, move ordering and the principal variation from the previous iteration.

### 8.1 Negamax

In chess, what is good for one side is equally bad for the other. Negamax uses this: a position's score for the side to move is the maximum, over its moves, of the negated score of the position after the move (which is scored for the opponent):

```text
score(position) = max over moves m of  −score(position after m)
```

Scoring every position from the side to move's view lets one function serve both players.

### 8.2 Alpha-beta pruning

Plain negamax visits every position up to the depth limit, which grows exponentially (about 35 moves per position: 35⁴ ≈ 1.5 million positions at depth 4). Alpha-beta prunes branches that cannot change the result, using two bounds:

- **alpha**: the score the side to move is already guaranteed elsewhere in the tree (it will not accept less);
- **beta**: the most the opponent will allow (if the side to move can get more, the opponent avoids this position entirely).

When a move scores at least beta, the remaining moves of that position need not be searched. This is a **beta cutoff**, and the opponent will never let the game reach the position. With good move ordering, alpha-beta searches roughly the square root of the minimax tree, so in the same time it reaches about twice the depth.

### 8.3 The search function

`Search::negamax(depth, ply, alpha, beta, hint, pv)` in `search/alphabeta.rs` searches the current position `depth` more plies (half-moves) deep. `ply` is the distance from the root. Step by step:

```text
negamax(depth, ply, alpha, beta, hint, pv):
    pv ← empty
    if ply > 0 and is_draw():               # repetition, fifty moves, insufficient material
        return 0
    if depth == 0 or ply ≥ 128:
        return evaluate(board)              # leaf: static evaluation
    moves ← legal_moves(board)
    if moves is empty:
        return −(MATE − ply) if in check    # checkmated: the nearer, the worse
               else 0                       # stalemate
    order moves (hint move first, then captures, then promotions)
    best ← −∞
    for each move:
        make_move; count node; push position hash
        score ← −negamax(depth − 1, ply + 1, −beta, −alpha, child hint, child pv)
        pop hash; unmake_move
        if the search must stop: return 0   # the caller discards this result
        if score > best:
            best ← score
            if score > alpha:
                alpha ← score
                pv ← move + child pv        # new best line
        if alpha ≥ beta: break              # beta cutoff
    return best
```

Note the window flip in the recursive call. The child is searched with `(−beta, −alpha)`, because the opponent's bounds are the mirror image of ours, and its score is negated on the way back.

### 8.4 Iterative deepening

`search` in `search/mod.rs` does not search to the final depth directly. It searches to depth 1, then 2, then 3, and so on. That looks wasteful but pays off:

- **It always has an answer.** When time runs out mid-iteration, the best move of the last completed depth is played.
- **It improves ordering.** The best line found at depth *d* (the principal variation) is searched first at depth *d + 1*, which makes alpha-beta cut far more. The shallow iterations cost little compared to the last one.
- **It produces telemetry.** Every completed depth is reported as an `info` line.

The loop ends when the depth limit is reached, when time or nodes run out ([§9](#9-time-management-and-stopping)), when a forced mate was found (more depth cannot change a mate within the horizon), or after depth 1 when there is only one legal move and a time budget exists.

Depth limits: `go depth N` searches to N (capped at 128). A bare `go` with no limit searches to depth 5. `go movetime`, a clock or `go nodes` deepen until the limit stops them.

### 8.5 The root

`search_root` searches the moves of the current position itself. It differs from `negamax` in three ways:

- it keeps the list of root moves between iterations and puts the previous best move first;
- it searches with an open window: alpha starts at −∞ and beta stays at +∞ at the root, and alpha rises as better moves are found;
- if the iteration is aborted, it returns nothing, so a half-searched iteration never replaces a complete one.

### 8.6 Move ordering

Alpha-beta cuts the most when the best move is searched first. `ordering.rs` gives every move a score and sorts them, highest first:

| Move | Score |
| --- | --- |
| The hint: the next move of the previous iteration's principal variation | 1,000,000 |
| Capture | 100,000 + 10 × victim value − attacker value |
| Promotion | 90,000 + value of the new piece (added to a capture's score) |
| Any other move | 0 |

Captures are ordered **MVV-LVA** (most valuable victim, least valuable attacker). Taking a queen comes before taking a knight, and taking a queen with a pawn comes before taking it with a rook, because the cheaper attacker risks less if the square is defended. Quiet moves keep their generation order.

The **hint** follows the previous principal variation down the tree. At the root it is the previous best move. At ply 1 it is the reply from that line, but only while the search is still on that line. Once the search leaves it, positions get no hint.

The effect is large. At depth 6 from the start position, plain minimax would visit about 124 million positions. The engine visits 950,000, including all the shallower iterations.

### 8.7 Mate scores

A checkmate is scored `−(MATE − ply)` for the mated side, with `MATE = 32000`. Being mated nearer to the root is worse. The winning side therefore prefers the fastest mate, and the losing side the slowest defeat. Any score beyond `MATE − 128` is a mate. For UCI it is converted to moves: `mate N` means the side to move mates in N moves, and `mate −N` means it gets mated in N. `mate 0` (no moves, in check) is reported only for a position that is already checkmated.

### 8.8 Draw detection

At every position below the root, before anything else, the search returns 0 (a draw) when:

- the halfmove clock reached 100 (the fifty-move rule);
- neither side can mate: no pawns, rooks or queens, and at most one knight or bishop on the board;
- the position already occurred, either earlier in the game or earlier on the current search line. Only positions since the last capture or pawn move are compared, because earlier ones cannot repeat.

A **single** repetition counts as a draw in the search, although the rules require three. If repeating once was the best either side could do, they can repeat again, so the outcome is the same. Treating it as a draw at once lets the engine see the draw earlier, steer into it when losing and away from it when winning. The game history comes from the `position … moves …` command, which is why the UCI layer records the hash of every position it replays.

Stalemate is detected when a position has no legal moves and the side to move is not in check (score 0).

### 8.9 Principal variation

Every call to `negamax` fills a `pv` vector with the best line from its position: the best move followed by the line its child returned. The root's line is reported as `pv` in the `info` output and used as the hint for the next iteration. Its first move is the move the engine plays.

## 9. Time management and stopping

### Time budget

`SearchLimits::time_budget` decides how long the side to move may think:

| `go` arguments | Budget |
| --- | --- |
| `movetime T` | T |
| `wtime/btime` (with optional `winc/binc`, `movestogo`) | `time / movestogo + 3/4 × increment`, where `movestogo` defaults to 30; never more than the clock minus 50 ms, and at least 1 ms |
| `infinite`, `ponder` | none: searches until `stop` |
| only `depth` or `nodes`, or nothing | none |

For example, `wtime 60000 winc 1000` gives White 60000 / 30 + 750 = 2750 ms.

### Two kinds of checks

1. **Before an iteration.** No new depth starts once half of the budget is used. The next iteration usually takes longer than all previous ones together, so starting it would almost certainly be wasted.
2. **During an iteration.** The search checks for `stop`, the node limit and the time budget. The node count is checked at every node. The clock and the stop flag are checked every 1024 nodes, because reading the clock is relatively slow. When a limit is hit, the search unwinds immediately and the unfinished iteration is discarded.

Limits are ignored until the first iteration completes. Even `go nodes 1` or a 1 ms budget therefore returns a legal move with a real depth-1 search.

### Stopping from outside

The UCI layer gives each search an `Arc<AtomicBool>` stop flag. `stop` sets it, the search returns its best move, and the UCI thread prints `bestmove`. For `go infinite` the search thread waits for `stop` before printing `bestmove`, even after finishing (for example, after a forced mate), as UCI requires.

## 10. Telemetry

After every completed depth the engine prints one line, for example:

```text
info depth 5 seldepth 5 score cp 70 nodes 104062 nps 365232 time 284 pv e2e4 b8c6 f1b5 c6d4 b5d7
```

| Field | Meaning in this engine |
| --- | --- |
| `depth` | Completed iteration depth in plies |
| `seldepth` | Deepest ply reached. Without extensions it equals `depth` |
| `score cp N` / `score mate N` | Evaluation for the side to move, in centipawns or moves to mate |
| `nodes` | Positions visited since the search started, all iterations together: one per `make_move` in the search |
| `nps` | Nodes per second |
| `time` | Milliseconds since the search started |
| `pv` | Best line found |

The arena stores the last line before `bestmove` with every move, and the frontend shows it in the move telemetry.

## 11. A search, step by step

The engine's real output for `position startpos` and `go depth 6`:

```text
info depth 1 seldepth 1 score cp 50 nodes 20 nps 606060 time 0 pv b1c3
info depth 2 seldepth 2 score cp 0 nodes 134 nps 219672 time 0 pv b1c3 b8c6
info depth 3 seldepth 3 score cp 50 nodes 1124 nps 348203 time 3 pv b1c3 b8c6 g1f3
info depth 4 seldepth 4 score cp 0 nodes 6644 nps 205843 time 32 pv b1c3 b8c6 g1f3 g8f6
info depth 5 seldepth 5 score cp 70 nodes 104062 nps 365232 time 284 pv e2e4 b8c6 f1b5 c6d4 b5d7
info depth 6 seldepth 6 score cp -45 nodes 949790 nps 325150 time 2921 pv e2e3 e7e5 g1e2 f8b4 e3e4 b4d2
bestmove e2e3
```

What it shows:

- **Depth 1** visits exactly the 20 legal moves and picks the one with the best piece-square gain (Nc3, +50).
- **Alpha-beta saves a lot.** Minimax to depth 4 would visit 20 + 400 + 8,902 + 197,281 = 206,603 positions. The engine visited 6,644 for depths 1 to 4 together. To depth 6 it visited 950,000 instead of about 124 million.
- **The score swings between odd and even depths** (+50, 0, +50, 0, +70, −45). At an odd depth White makes the last move, and its gain is counted with no reply. At an even depth Black answers. Without a quiescence search ([§12](#12-known-weaknesses)), the evaluation at the horizon depends on who moved last.
- **The final choice can be odd** (1.e3 at depth 6). At an even depth every line ends with Black's move, and here the principal variation ends with 6…Bxd2+, a capture whose recapture lies beyond the horizon. Lines that end in an unanswered capture distort the comparison between White's first moves.
- **Each depth costs about 5–15 times the previous one**, so one more ply needs roughly ten times the time. That is why 1 second per move reaches depth 5–6.

## 12. Known weaknesses

Ordered by how much fixing them is expected to improve playing strength.

### No quiescence search: the horizon effect

The search stops at a fixed depth and evaluates the position statically, even in the middle of an exchange. If the last move was a capture, the recapture one ply later is not seen. A real example from the arena, after 1.e4 e5 2.Nf3 Nc6 3.Bb5 a6 with 1 second per move:

```text
info depth 5 … score cp 170 … pv b5c6 d7c6 f3e5 g8f6 e5f7
```

The engine plans 4.Bxc6 dxc6 5.Nxe5 and expects to keep the extra pawn. It does not see 5…Qd4, which attacks the knight on e5 and the pawn on e4 at once, after which Black wins the pawn back (6.Nf3 Qxe4+). The usual fix is a **quiescence search**: at depth 0, keep searching captures (and possibly checks) until the position is quiet, with "stand pat" (the static evaluation) as a lower bound. It also removes most of the odd/even score swings.

### No transposition table

The same position is often reached through different move orders (1.e4 e5 2.Nf3 and 1.Nf3 e5 2.e4). Each time, the engine searches it again from scratch. A transposition table, a hash table keyed by the position hash, stores the score, depth, bound type and best move of searched positions. It skips repeated work and supplies a strong first move for ordering at every position, not only along the principal variation.

### Simple move ordering for quiet moves

Captures are ordered well, but quiet moves keep generation order. **Killer moves** (quiet moves that caused a beta cutoff at the same ply in a sibling position) and the **history heuristic** (quiet moves that caused cutoffs anywhere, weighted by depth) usually reduce the tree substantially.

### Slow move generation and attack tests

About 200,000–600,000 nodes per second is slow for a bitboard engine; others reach millions. The main costs are:

- legality by make/unmake plus a full attack test for every pseudo-legal move;
- `is_square_attacked` scanning all 64 squares and building every enemy piece's attacks;
- `find_king` scanning the board instead of reading the king's bitboard;
- the position hash recomputed from scratch at every node;
- move lists allocated as new `Vec`s at every node.

Precomputed attack tables, magic bitboards for sliding pieces, checking attacks backwards from the king's square ("is a knight standing a knight's jump away?"), an incremental hash and fixed-size move lists would make the same search several times faster. That means one or two plies deeper in the same time.

### Search refinements

- **Checkmate at the horizon is not seen.** At depth 0 the position is evaluated statically even if it is checkmate, so a mate on the last ply only counts as material. A quiescence search that detects positions without legal moves fixes this.
- **No check extensions.** A check could extend the search by one ply, so forcing lines are followed to their end.
- **No pruning or reductions**: null-move pruning, late move reductions and futility pruning are the standard ways to search the important moves deeper.
- **Full window at the root.** Aspiration windows (a narrow window around the previous score) and principal variation search (zero-window searches for moves after the first) cut more.

### Evaluation

Only material and piece-square tables are counted (see [§7](#what-the-evaluation-does-not-know)). The first additions with the biggest return are usually passed pawns, the bishop pair, rook on open files, mobility and simple king safety, followed by tuning the values on game data.

### Measuring improvements

Every change above should be checked with the arena, not by intuition:

```powershell
./scripts/regress.ps1 -Candidate 1.3.0 -Baseline 1.2.0
```

The script plays an SPRT match on the opening suite and reports whether the new version is stronger (see [Analysis and statistics](analysis-and-statistics.md)). Stockfish's per-move analysis (accuracy, centipawn loss, blunders) shows *where* a version still goes wrong, for example how often it blunders in tactical positions, which is where quiescence search should help first.
