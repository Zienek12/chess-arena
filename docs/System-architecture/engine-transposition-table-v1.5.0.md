# Transposition Table (version 1.5.0)

Version 1.5.0 adds a transposition table: a hash table that remembers what the search found about each position. The search no longer repeats work for a position it has already searched deeply enough, and it tries the move that was best last time first.

For the search it extends, see [Engine internals (v1.2.0)](engine-internals-v.1.2.0.md#8-search) and [Quiescence search (v1.4.0)](engine-quiescence-v1.4.0.md).

## The problem: transpositions

The same position is often reached by different move orders, called transpositions. 1.e4 e5 2.Nf3 and 1.Nf3 e5 2.e4 lead to one position, yet a search without memory evaluates it separately each time. In endgames, where few pieces shuffle around, the same positions occur a huge number of times.

Iterative deepening repeats work as well. Each iteration visits almost all positions of the previous one. Until now only the principal variation was carried over, so every other position had to find its best move again from scratch.

## The table

`search/tt.rs` implements a table with one slot per index. The index is the low bits of the position's 64-bit Zobrist hash, the one the board already keeps up to date since [version 1.3.0](engine-performance-v1.3.0.md#5-incremental-position-hash).

| Field | Size | Meaning |
| --- | ---: | --- |
| Hash | 64 bits | The whole position hash, to recognize that the slot holds this position |
| Score | 16 bits | The result of the search |
| Depth | 8 bits | How deep the position was searched |
| Bound | 8 bits | What the score means, see below |
| Best move | 16 bits | The source square, target square and promotion piece |

One slot takes 16 bytes. The default of 64 MiB holds about 4 million positions. The size can be changed with the UCI option `Hash` (1 to 1024 MiB), for example `setoption name Hash value 128`. The arena limits an engine container to 256 MB by default, so the table must stay well below that.

The memory is requested already zeroed from the operating system, and an all-zero slot means empty. The system supplies the pages only when they are first written, so creating and clearing the table costs almost nothing and does not use up the time for the first move.

### Bounds

Because of alpha-beta pruning, a search does not always find a position's exact score:

| Result of the search in window `alpha..beta` | Bound | Meaning |
| --- | --- | --- |
| Best score at least `beta` (a cutoff; the remaining moves were skipped) | Lower | The true score is at least the stored score |
| Best score at most `alpha` (all moves failed) | Upper | The true score is at most the stored score |
| Best score between them | Exact | The stored score is the true score |

### Replacement

A new result replaces whatever is in the slot, with one exception: a deeper result for the same position is kept. This scheme is simple and works without tracking the age of entries, since an old deep entry of another position is overwritten at once.

## Using the table in the search

At every `negamax` node, after the draw checks:

```text
entry ← table.probe(position hash)
if entry was searched at least to depth:
    if bound allows it and score ≤ alpha, or score ≥ beta:
        return score                    # the moves need not be searched
...
order moves: PV hint, then the entry's best move, then captures, promotions, quiet moves
search the moves as before
table.store(hash, depth, best score, bound, best move)
```

### Why a score inside the window is not used

A stored score is returned only if it falls outside the current window. Such a node cannot raise `alpha` in its parent, so it is never part of the principal variation. A score inside the window would end the search of a principal-variation node, and the line reported as `pv` would be cut short at that position. Searching that node normally keeps `pv` complete. Principal-variation nodes are a tiny part of the tree, so this costs little.

### Mate scores

The search scores a mate by its distance from the root: `MATE − ply`. The same position can occur at different plies, and a stored score must be right for all of them. The table therefore stores mate scores counted from the position itself and converts them on the way in and out:

```text
stored = score + ply     (when storing a winning mate; − ply for a losing one)
score  = stored − ply'   (when reading it at another ply)
```

Without this conversion the engine would report wrong mate distances, and a longer mate could look shorter than a real short one.

### What is not stored

- **Quiescence nodes.** They are numerous, shallow and quick to redo. The table is used only at depth 1 or more.
- **The root.** The root moves are kept between iterations in `search_root`.
- **Aborted searches.** A node that stops because of a limit returns before storing, so an unfinished result can never enter the table.

### Known limitations

The hash identifies the pieces, side to move, castling rights and en passant, but not how the position was reached. Two results can therefore be wrong in rare cases:

- **Repetitions.** A score may include a draw by repetition that depends on the moves leading to the position. This is called the graph history interaction problem. Repetition draws are still checked before the table at every node, so the engine does not walk into a repetition it can see directly.
- **The fifty-move rule.** A stored score does not know how close the counter is to 100.

Most engines accept both. A **hash collision**, two positions sharing 64 bits, is so unlikely that it can be ignored. It could only produce a wrong score, not an illegal move: a stored move is used solely for ordering, after it is found in the list of legal moves.

### Between moves of a game

The table stays between the moves of a game, because the position after the opponent's reply was usually searched already. `ucinewgame` clears it. The bench creates a new table of a fixed size for each position, so that its node count depends on nothing but the search.

## Results

The node counts below are deterministic and can be repeated on any computer. For the same depth the search must search fewer positions, and in all four positions it also returns the **same score**, which is a strong sign that the table does not introduce errors.

| Position and depth | 1.4.0 nodes | 1.5.0 nodes | Reduction | Score (both) |
| --- | ---: | ---: | ---: | ---: |
| Start position, depth 7 | 5,320,605 | 1,343,933 | 4.0× | +35 |
| Perft position 6 (middlegame), depth 7 | 7,458,918 | 2,844,580 | 2.6× | +15 |
| Kiwipete, depth 6 | 1,682,239 | 740,107 | 2.3× | +30 |
| Fine #70 (pawn endgame), depth 14 | 4,411,128 | 9,673 | 456× | +175 |

| Measurement | 1.4.0 | 1.5.0 |
| --- | ---: | ---: |
| Bench signature (depth 5) | 2,492,336 | 1,405,750 |

Fine #70 (`8/k7/3p4/p2P1p2/P2P1P2/8/8/K7 w - - 0 1`) is a classic test for transposition tables: only the kings can move for many plies, so nearly every position can be reached in many ways.

Nodes per second are lower than in 1.4.0, because each node also reads and writes a table in memory that does not fit in the processor's cache. Fewer nodes per depth matter more than this cost, but only a time-based measurement shows the net gain, and such a measurement must be taken while the computer is otherwise idle. The strength result is the SPRT tournament:

```powershell
./scripts/regress.ps1 -Candidate 1.5.0 -Baseline 1.4.0
```

## Verification

- The tests in `search/tt.rs` cover storing and reading all fields, bounds on each side of the window, an exact score inside the window, depth, replacement, mate score conversion across plies, and telling promotions apart.
- The tests in `search/mod.rs` search one position twice with the same table: the second search needs far fewer nodes, the principal variation stays complete, and a mate keeps its correct distance.
- The tests in `uci.rs` cover the `Hash` option, invalid values, and that `ucinewgame` makes a search repeat its node count exactly.

## What is still missing

- **Buckets or two-tier replacement:** several positions per index, so a deep result is not lost to a shallow one of another position.
- **The table in quiescence search**, once its cost has been measured.
- **Narrowing the window** with a stored bound, instead of using it only for a cutoff.
- **Age of entries:** preferring to overwrite entries from earlier moves of the game.
