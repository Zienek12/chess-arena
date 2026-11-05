# Null Move Pruning and Late Move Reductions (version 1.7.0)

Version 1.7.0 makes the search selective. Earlier versions skipped only moves that provably could not change the result. This one also searches unpromising moves less deeply, on the expectation that they do not matter. The saved time buys more depth in the lines that do matter.

For the search it extends, see [Engine internals (v1.2.0)](engine-internals-v.1.2.0.md#8-search) and [Killer moves and history heuristic (v1.6.0)](engine-move-ordering-v1.6.0.md).

## The problem: full depth for every move

Alpha-beta never changes the result of the search it prunes, but that guarantee has a price. Every move that is not pruned gets the full depth, including the many that are obviously bad. The tree still grows by a large factor per ply, so a position with 35 moves spends almost all its time on moves no strong player would consider.

Selective techniques give up the guarantee. They can miss a move, unlike the pruning of earlier versions, so their value can only be shown by playing games.

## Null move pruning

If a position is so good that the side to move stays above `beta` even after passing its turn, a real move should do even better, because having a move is nearly always an advantage. The opponent gets two moves in a row, and if that does not help, the node is cut off:

```text
if conditions hold:
    pass the turn
    score ← −search(depth − 1 − R, window −beta .. −beta + 1)
    take the pass back
    if score ≥ beta:
        return score
```

The reduction `R` is `2 + depth / 6`, so the test costs only a small part of the search it can save.

### Conditions

| Not tried when | Reason |
| --- | --- |
| Less than 3 plies of depth remain | The reduced search would prove too little |
| At the root, or on the principal variation of the previous iteration | Those are the moves the engine will actually play |
| The side to move is in check | Passing would be illegal |
| The side has only a king and pawns | Zugzwang is common there, and passing may be the only "good" move |
| The static evaluation is below `beta` | A cutoff is unlikely, so the search would be wasted |
| `beta` is a mate score | The test could not prove it |

The zugzwang condition is essential. In a position where every move worsens the position, passing looks better than any real move, and the engine would cut off a position that is actually lost. It does not cover every zugzwang: such positions also exist with pieces on the board, but they are rare.

Two passes in a row need no separate check. The evaluation is exactly the opponent's with the sign reversed. After a pass at `beta`, the side to move has an evaluation of at most `−beta`, below its own beta of `−beta + 1`, so it does not pass again.

### Details

- A pass (`Board::make_null_move`) changes the side to move, clears the en passant square and restarts the halfmove clock. Restarting the clock means no position before the pass counts as a repetition, because a line with a pass in it can never occur in a game.
- A returned mate score is capped below the mate range. A mate found after passing is not a real line, so its distance means nothing.
- The cutoff is not stored in the transposition table, because no real best move goes with it.

## Late move reductions

After good move ordering, a move late in the list rarely turns out best. Late move reductions (LMR) first search it less deeply, with a window that only asks whether it beats `alpha`. Only if it does was the reduction wrong, and the move then gets the full search:

```text
for each move, with its index in the ordered list:
    if conditions hold:
        score ← −search(depth − 1 − reduction, window −alpha − 1 .. −alpha)
        if score ≤ alpha:
            continue                # as expected: the move is not good enough
    score ← −search(depth − 1, window −beta .. −alpha)
```

This depends directly on versions 1.5.0 and 1.6.0: the better the ordering, the safer it is to trust the position of a move in the list.

### Conditions

A move is reduced only if at least 3 plies of depth remain, it is the fourth move or later, it is quiet, the side to move is not in check, and the move does not give check. Captures, promotions and checks are the moves most likely to change the assessment, so they keep the full depth.

### The size of the reduction

```text
reduction = 1 + ⌊log2(depth)⌋ × ⌊log2(index + 1)⌋ / 4     (at most depth − 2)
```

Later moves and greater depths are reduced more, and at least one ply always remains. Integer logarithms are used because the same result on every platform keeps the bench signature identical. Floating-point functions need not round the same way everywhere.

## Results

### What "the same depth" means now

Up to version 1.6.0, the same depth meant the same search with less work. That no longer holds: at a given depth the search now looks less deeply at many moves, so a nominal depth is worth less than before. Node counts at one depth therefore cannot be compared the way they were for earlier versions, and the score or the best move can change. In the start position at depth 7, for example, 1.6.0 reports `e2e4` with +35 and 1.7.0 reports `b1c3` with +20.

| Position and depth | 1.6.0 nodes | 1.7.0 nodes | Score (1.6.0 / 1.7.0) | Best move (1.6.0 / 1.7.0) |
| --- | ---: | ---: | --- | --- |
| Start position, depth 7 | 249,086 | 11,074 | +35 / +20 | `e2e4` / `b1c3` |
| Perft position 6, depth 7 | 2,258,007 | 286,363 | +15 / +15 | `c3d5` / `c3d5` |
| Kiwipete, depth 6 | 733,572 | 288,167 | +30 / +30 | `d5e6` / `d5e6` |
| Fine #70, depth 14 | 6,709 | 5,279 | +175 / +175 | `a1b2` / `a1b2` |
| Bench signature (depth 5) | 1,008,987 | 385,276 | | |

### Depth in the same time

The depth completed with `go movetime 1000` is the fairer measure. The table gives the range over three runs:

| Position | 1.6.0 | 1.7.0 |
| --- | ---: | ---: |
| Start position | 7 | 11 |
| Perft position 6 | 5 | 7 |
| Kiwipete | 5 | 6 |
| Perft position 3 (rook endgame) | 10 | 14–15 |

Selectivity did not delay the solution of the classic Fine #70 pawn endgame either, where the zugzwang condition keeps null move pruning off. Both versions first choose the winning `a1b1` at depth 24, with 40,236,986 nodes in 1.6.0 and 3,696,952 in 1.7.0.

Nodes per second fall by about 12% (1.61 to 1.42 million on the bench, best of 7 alternating runs on one computer under the same background load). The likely cause is that the tree now has relatively fewer cheap leaf nodes and more interior ones, which also test for check and may evaluate. The tree shrinks far more than that.

### The contribution of each part

These variants were built only for this comparison:

| Variant | Bench | Start, depth 7 | Perft 6, depth 7 | Kiwipete, depth 6 |
| --- | ---: | ---: | ---: | ---: |
| 1.6.0 | 1,008,987 | 249,086 | 2,258,007 | 733,572 |
| Null move only | 805,928 | 102,447 | 740,010 | 665,621 |
| LMR only | 439,607 | 28,467 | 592,946 | 294,038 |
| **Both (1.7.0)** | **385,276** | 11,074 | 286,363 | 288,167 |

Node counts alone do not show which part adds strength, so short matches were played at 100 ms per move from 12 openings. They only guided decisions during development and are not a strength measurement:

| Match | Games | Result | Score |
| --- | ---: | ---: | ---: |
| 1.7.0 vs 1.6.0 | 48 | +23 =17 −8 | 65.6% |
| Null move only vs 1.6.0 | 48 | +26 =13 −9 | 67.7% |
| LMR only vs 1.6.0 | 48 | +22 =12 −14 | 58.3% |
| 1.7.0 vs null move only | 192 | +90 =50 −52 | 59.9% |
| Reduction of 1 for all late moves vs 1.7.0 | 48 | +15 =17 −16 | 49.0% |
| Fixed null move `R = 2` vs 1.7.0 | 48 | +19 =13 −16 | 53.1% |

The 48-game results for the two parts could not separate them, so the 192-game match decided that LMR stays. The two simpler parameter settings showed no clear difference from the chosen ones, so these values were not tuned further. The strength result is the SPRT tournament:

```powershell
./scripts/regress.ps1 -Candidate 1.7.0 -Baseline 1.6.0
```

## Verification

- The tests in `board.rs` check that a pass changes the expected fields and is undone exactly, and the test for non-pawn material.
- The tests in `search/alphabeta.rs` cover the null move conditions, the impossibility of two passes in a row, a cutoff in a winning position, no cutoff when passing allows mate, and the limits of the reduction formula.
- The tests in `search/mod.rs` find a mate that begins with a quiet queen sacrifice, which LMR must not lose, and check that the principal variation stays complete.

## Known risks

- **Zugzwang with pieces on the board** can still mislead null move pruning. Verification search, a reduced search without passing that confirms the cutoff, is the usual remedy.
- **A reduced move can hide a deep tactic.** The search sees it only if the reduced search already beats `alpha`, or at a later iteration.
- **Non-principal-variation nodes cannot be told apart reliably.** This version does not use principal variation search, so most nodes have a wide window, which limits how often null move pruning applies and when a reduction can be chosen more aggressively.

## What is still missing

- **Principal variation search**, zero-window searches for all moves after the first.
- **Aspiration windows** at the root, around the score of the previous iteration.
- **Check extensions** and **futility pruning**.
- **A reduction that depends on the history score**, reducing moves with a poor history more.
- **Evaluation terms** beyond material and piece-square tables (pawn structure, mobility, king safety).
