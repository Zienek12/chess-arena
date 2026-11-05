# Killer Moves and History Heuristic (version 1.6.0)

Version 1.6.0 orders quiet moves by what the search has learned. Until now quiet moves kept the order of the move generator, so in a position without a good capture the refuting move was found only by chance. The engine now tries first the quiet moves that recently caused beta cutoffs.

For the ordering it extends, see [Engine internals (v1.2.0)](engine-internals-v.1.2.0.md#86-move-ordering) and [Transposition table (v1.5.0)](engine-transposition-table-v1.5.0.md).

## The problem: quiet moves have no static hint

Alpha-beta cuts the most when the best move comes first. A capture can be judged before searching it: taking a queen with a pawn is promising. A quiet move gives no such hint, yet most positions in the tree have no good capture, and in many of them a quiet move is the refutation. Every quiet move searched before it is wasted work, each with its whole subtree.

A refutation is also often the same move in many positions. If a quiet move answers one line by threatening mate, it probably answers the sibling lines as well, because the moves between them changed little. The search can therefore learn during its run, with no chess knowledge added.

## The new order

| Move | Score |
| --- | --- |
| Hint: the next move of the previous iteration's principal variation | 1,000,000 |
| Best move stored in the transposition table | 500,000 |
| Capture | 100,000 + 10 × victim value − attacker value |
| Promotion | 90,000 + value of the new piece (added to a capture's score) |
| First killer of this ply | 80,000 |
| Second killer of this ply | 79,000 |
| Other quiet move | Its history score, from −16,384 to 16,384 |

A move counts as quiet if it is neither a capture nor a promotion, which includes pawn pushes and castling. Moves with equal scores keep the order of the move generator.

## Killer moves

`search/heuristics.rs` keeps two killer moves for each ply: the last two different quiet moves that caused a beta cutoff at that ply. The positions at one ply are often siblings, so a move that worked in one has a good chance in the next.

A killer needs no validation. It only raises the score of a move already in the list of legal moves, so a killer that is not legal in this position has no effect.

Two slots are kept so that one refutation seen in several lines is not lost when another line briefly needs a different one. A move that is already the first killer does not shift the second out.

## History heuristic

The history is a table of scores indexed by side to move, source square and target square, shared by all plies. After a beta cutoff by a quiet move at remaining depth `d`, with `bonus = d²`:

```text
cutoff move:                 reward by bonus
quiet moves tried before it: penalize by bonus
```

A deep cutoff counts more, since it rests on a larger search and saves more work. The penalty matters because a move that is often tried and rarely works should fall behind moves never tried.

Scores are updated so that they cannot leave their range:

```text
score ← score + change − score × |change| / 16384
```

The nearer a score is to the limit, the smaller the step in that direction. A move that kept working can therefore be overtaken once it stops working, since recent results weigh more than old ones.

### What is not learned from

- **Captures and promotions.** They are already ordered, and a capture's success says little about a quiet move.
- **Cutoffs from the transposition table.** No moves are searched there, so there is no move list to learn from.
- **Aborted searches.** A node that stops because of a limit returns before the update.
- **Quiescence search and the root.** Quiescence search has its own ordering and few quiet moves. Beta at the root is infinite, so no cutoff occurs there, and root moves keep their order from the previous iteration.

## Lifetime

Killers and history are created empty at the start of each `go`. They last through all iterations of that search, which is where most of their value comes from: what one iteration learned orders the next.

They are not kept between the moves of a game, unlike the transposition table. So a search depends only on its position and the table, the bench stays deterministic, and `ucinewgame` needs to clear nothing new.

## Results

The node counts below are deterministic and can be repeated on any computer. Move ordering changes which nodes are visited, not the value alpha-beta computes, so the score should stay the same. In all four positions both the score and the best move are the same as in 1.5.0. The table and repetition detection depend on the search path, so this is not guaranteed in every position.

| Position and depth | 1.5.0 nodes | 1.6.0 nodes | Reduction | Score (both) |
| --- | ---: | ---: | ---: | ---: |
| Start position, depth 7 | 1,343,933 | 249,086 | 5.4× | +35 |
| Perft position 6 (middlegame), depth 7 | 2,844,580 | 2,258,007 | 1.26× | +15 |
| Kiwipete, depth 6 | 740,107 | 733,572 | 1.01× | +30 |
| Fine #70 (pawn endgame), depth 14 | 9,673 | 6,709 | 1.44× | +175 |
| Bench signature (18 positions, depth 5) | 1,405,750 | 1,008,987 | 1.39× | |

The gain depends on the position. In the quiet start position the refutations are quiet moves, so learning helps a lot. Kiwipete is full of captures that MVV-LVA already orders well, so almost nothing is gained there.

### The contribution of each part

These variants were built only for this comparison:

| Variant | Bench | Start, depth 7 | Perft 6, depth 7 | Kiwipete, depth 6 | Fine #70, depth 14 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1.5.0 | 1,405,750 | 1,343,933 | 2,844,580 | 740,107 | 9,673 |
| Killers only | 1,024,246 | 314,395 | 2,339,388 | 734,598 | 6,948 |
| History only | 1,025,047 | 239,660 | 2,265,382 | 733,814 | 6,786 |
| Both, without the history penalty | 1,014,060 | 282,938 | 2,261,090 | 734,811 | 6,825 |
| **Both (1.6.0)** | **1,008,987** | 249,086 | 2,258,007 | 733,572 | 6,709 |

Either part alone gives most of the gain, because both promote the same refuting moves. Together they are best on the bench and in three of the four positions. In the start position history alone visits about 4% fewer nodes than both. The differences between the variants are small compared with the step from 1.5.0.

### Speed and strength

Speed is nearly unchanged. The bench reached 2.21 million nodes per second in 1.5.0 and 2.18 million in 1.6.0 (best of 7 runs each, on one otherwise idle computer), so the extra work per node costs about 1%, well below the reduction in nodes.

Fewer nodes for the same depth do not yet prove greater strength. That is measured by an SPRT tournament:

```powershell
./scripts/regress.ps1 -Candidate 1.6.0 -Baseline 1.5.0
```

## Verification

- The tests in `search/heuristics.rs` cover replacing killers, a repeated killer, killers belonging to one ply, the reward and penalty, ignoring captures, the separate history of each side, and the score limits.
- The tests in `search/ordering.rs` check the whole order: capture, promotions, killer, history, unknown quiet moves, and a penalized move last.
- The tests in `search/alphabeta.rs` run `negamax` and check that a quiet cutoff is learned from, while a capture cutoff or a search without a cutoff teaches nothing.

## What is still missing

- **Static exchange evaluation:** a losing capture such as queen takes defended pawn is still tried before every killer.
- **Countermove heuristic:** a quiet move remembered as the answer to the opponent's previous move.
- **Continuation history:** history indexed also by the previous move, instead of by squares alone.
- **Keeping the history between moves of a game**, if a measurement shows a gain.
- **History in quiescence search**, for ordering the evasions of a side in check.
