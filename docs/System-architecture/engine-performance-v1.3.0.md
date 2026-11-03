# Engine Performance (version 1.3.0)

Version 1.3.0 makes the engine about ten times faster without changing a single decision it makes. Every position is evaluated the same way, every move list comes out in the same order, and the search visits exactly the same positions as in 1.2.0. Only the time per position is shorter, so in the same time the engine searches one or two plies deeper.

For how the engine worked before, see [Engine internals (v1.2.0)](engine-internals-v.1.2.0.md). This document describes what changed and how much each change gained.

## Results

### Method

The [bench](engine-implementation.md#bench) searches 18 fixed positions to depth 5. Its node count, the **bench signature**, depends only on what the search does. It stayed at **1,390,581** after every step below, which shows that no step changed the engine's behavior. Each step was added on top of the previous ones and measured as the best of seven bench runs on the same Windows 11 computer. Absolute speeds differ between computers; the ratios are what matter.

| Step | Change | Nodes/second | Speed-up |
| --- | --- | ---: | ---: |
| A | Version 1.2.0 | 275,861 | 1.0× |
| B | Precomputed knight, king and pawn attacks; magic bitboards for sliding pieces | 581,325 | 2.1× |
| C | Legality from pins instead of playing every move; king found from its bitboard | 1,248,578 | 4.5× |
| D | Incremental position hash | 1,859,505 | 6.7× |
| E | Move ordering without heap allocation | 2,471,594 | 9.0× |
| F | Move lists on the stack (`MoveList`) | 2,750,515 | 10.0× |

### Other measurements

| Measurement | 1.2.0 | 1.3.0 |
| --- | ---: | ---: |
| Perft, Kiwipete depth 5 (193,690,690 leaves) | 150 s | 4.9 s |
| Depth reached with `go movetime 1000`, start position | 5 | 6 |
| Depth reached with `go movetime 1000`, middlegame (perft position 6) | 4 | 6 |
| Bench signature | 1,390,581 | 1,390,581 |

Perft only generates moves and plays them, so it gains even more than the search: about 30 times.

## 1. Precomputed attacks of pawns, knights and kings

In 1.2.0, `knight_attacks(square)` computed the knight's targets from its coordinates on every call: eight offsets, each checked against the board edge. A knight on a given square always attacks the same squares, however, so 1.3.0 computes all 64 results once and reads them from a table. Kings and pawns (one table per color) work the same way.

The tables are built by a `const fn` while the engine is compiled, so they cost nothing at runtime:

```rust
static KNIGHT_ATTACKS: [u64; 64] = leaper_table(&KNIGHT_OFFSETS);
```

## 2. Magic bitboards

Bishops, rooks and queens are harder: their attacks depend on which squares are occupied, because each ray stops at the first piece. In 1.2.0 they were found by walking each ray one square at a time. Magic bitboards replace that walk with one multiplication and one table lookup.

### The idea

1. **Only some squares matter.** A rook on d4 can be blocked only by pieces on its rays, and not by a piece on the last square before the edge, since nothing stands behind it. These squares form the rook's *relevant mask*: 10 squares for a rook on d4, 12 for a rook in a corner, 5 to 9 for a bishop.
2. **Every blocker set can be listed.** With 10 relevant squares there are 2^10 = 1024 possible blocker sets, each with its own attack set. All of them can be computed in advance with the slow ray walk.
3. **They need an index.** The blocker set is a 64-bit number with up to 12 scattered bits. It must be mapped to a small index into the table of precomputed attacks.

The mapping is a multiplication by a *magic number*:

```text
index = ((occupancy & mask) × magic) >> (64 − bits)
attacks = table[offset + index]
```

Multiplying spreads copies of the mask's bits across the 64-bit product. With a well-chosen magic, the top `bits` bits of the product are different for every blocker set that has different attacks. Two blocker sets may share an index only if their attacks are the same; such collisions are harmless and even make the table smaller.

### Finding the magics

There is no formula for magic numbers. They are found by trial: take a random number with few set bits (the AND of three random numbers), compute the index of every blocker set, and accept the number if no two sets with different attacks collide. Many numbers are usually rejected before one works.

This search takes about half a second, too long to repeat every time the engine starts, because the arena starts a new engine process for every game. The magics were therefore found once by the test `find_magics` and are stored as constants (`ROOK_MAGICS`, `BISHOP_MAGICS` in `attacks.rs`). To generate them again:

```powershell
cargo test --release find_magics -- --ignored --nocapture
```

### The tables

When the engine starts, `attacks::init()` fills the tables from the stored magics, using the ray walk once for every blocker set. This takes a few milliseconds.

| Piece | Entries | Size |
| --- | ---: | ---: |
| Rook (64 squares, 2^10 to 2^12 each) | 102,400 | 800 KiB |
| Bishop (64 squares, 2^5 to 2^9 each) | 5,248 | 41 KiB |

A queen's attacks are the union of a rook's and a bishop's from the same square.

### Verification

The test `magics_give_the_attacks_of_every_blocker_set` compares the table lookup with the ray walk for every square and every possible blocker set: all 107,648 entries. Another test compares them on random full boards. Perft then confirms that the move generator built on the tables produces exactly the reference move counts.

## 3. Attack test from the target square

Every legality check asks whether a square, usually the king's, is attacked. In 1.2.0, `is_square_attacked` looped over all 64 squares and computed the attacks of every enemy piece it found.

1.3.0 uses the symmetry of attacks: a knight on b1 attacks d2 exactly when a knight on d2 would attack b1. So it computes each kind of attack once, *from the target square*, and checks whether it hits an enemy piece of that kind:

```text
attacked(square) =
       knight_attacks(square)       & enemy knights
    or king_attacks(square)         & enemy king
    or pawn_attacks(square, us)     & enemy pawns       # pawns are not symmetric: use our color's pawn attacks
    or bishop_attacks(square, occ)  & enemy bishops and queens
    or rook_attacks(square, occ)    & enemy rooks and queens
```

That is five table lookups instead of a scan of the board. The king's square is also read from the king bitboard (`trailing_zeros`) instead of scanning the board for it.

## 4. Legal moves without playing every move

1.2.0 decided whether a move was legal by playing it, testing whether the own king was attacked, and taking it back. This is always correct but expensive, and almost always unnecessary: in most positions nearly every move is legal.

1.3.0 first computes two facts about the position:

- whether the king is **in check**;
- which own pieces are **pinned**: standing between the own king and an enemy bishop, rook or queen on the same line, with nothing else in between. To find them, the king "looks" along the rook and bishop lines, removes the own pieces it sees first, and looks again. An enemy slider that appears behind a removed piece pins that piece.

A move can then be classified without playing it:

| Move | Legal when |
| --- | --- |
| King move | its target is not attacked, with the king removed from the board so that it cannot shield its own target square from a slider |
| Any other move, king not in check, piece not pinned | always |
| Castling, en passant, a pinned piece, or any move while in check | decided by playing the move, as before |

The last row covers the rare cases. En passant is one of them because it removes two pawns from one rank at once, which can expose the king in a way the pin test does not see.

The result is a *filter* over the same pseudo-legal list, so the legal moves come out in the same order as in 1.2.0 (see [Why the order of moves must not change](#why-the-order-of-moves-must-not-change)).

## 5. Incremental position hash

The search computes the position hash at every node to detect repetitions. In 1.2.0 the hash was computed by scanning all 64 squares.

The hash is an XOR of one key per piece on its square, plus keys for the side to move, castling rights and en passant. XOR can be undone by repeating it, so the board now keeps the pieces' part up to date as pieces move: `set_piece` and `clear_square` XOR the piece's key in and out. `position_hash` only adds the three small keys. The Zobrist keys are the same as in 1.2.0, now precomputed at compile time, so every position has the same hash as before.

`unmake_move` restores the hash automatically, because it puts the pieces back with the same two functions. A test walks three plies of the move tree from positions with castling, en passant and promotions and checks at every node that the incremental hash equals one computed from scratch.

## 6. No heap allocation per node

At around a million nodes per second, even one memory allocation per node is noticeable.

- **Move ordering** used `sort_by_cached_key`, which allocates a buffer for the keys. It is replaced by an insertion sort that keeps the scores in an array on the stack. Insertion sort is stable and fast for the few dozen moves of a position.
- **Move lists** were a `Vec<Move>` allocated at every node. `MoveList` is a fixed array of 256 moves on the stack with a length. The most moves any known legal position has is 218. It dereferences to a slice, so the rest of the code uses it like the `Vec` before.

## Why the order of moves must not change

Alpha-beta pruning depends on the order in which moves are searched. Move ordering sorts captures and the principal-variation move to the front, but all quiet moves get the same score. A stable sort keeps them in the order the generator produced. If that order changed, the search would visit different positions, the node count would change, and in some positions it would even pick a different move.

The new code therefore keeps the order of 1.2.0 everywhere:

- pieces are visited from a1 to h8, now by iterating the own pieces' bitboard from its lowest bit, and their targets in the same order;
- the legality test removes illegal moves from the list but never reorders it;
- the new insertion sort is stable, like the sort it replaces.

The unchanged bench signature confirms this. It is what makes the comparison fair: 1.2.0 and 1.3.0 play the same moves at the same depth, so any difference in a tournament comes from speed alone.

## What is still slow

The engine is still several times slower than mature bitboard engines. The remaining costs, in rough order of size:

- **Evaluation from scratch.** `evaluate` loops over all pieces at every leaf. Material and piece-square scores could be updated incrementally in `make_move`, like the hash.
- **Full move generation before a cutoff.** All legal moves are generated and sorted, even when the first move causes a beta cutoff. Staged generation produces the hint move first and captures next, and generates quiet moves only when they are needed.
- **Check evasions** are still tested by playing each move.
- **`make_move`** looks up castling rights in a loop over four corners on every move.

These changes keep the search unchanged too, but the next versions focus on making the search itself smarter, starting with quiescence search (see [Known weaknesses](engine-internals-v.1.2.0.md#12-known-weaknesses)).

## Effect on playing strength

A faster engine reaches greater depth in the same time, and greater depth means stronger play. How much stronger has to be measured with an SPRT tournament against 1.2.0:

```powershell
./scripts/regress.ps1 -Candidate 1.3.0 -Baseline 1.2.0
```

Starting a Docker container adds latency to every move, especially on Docker Desktop for Windows. The engine's own speed-up is therefore best seen with longer time controls, where the search time is large compared to that overhead.
