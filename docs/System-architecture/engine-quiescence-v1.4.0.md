# Quiescence Search (version 1.4.0)

Version 1.4.0 adds a quiescence search: at the end of the normal search, the engine keeps playing captures until the position is quiet, and only then evaluates it. This removes the engine's largest weakness, the horizon effect. It is the first version since 1.2.0 that changes which moves the engine plays, so its bench signature is new.

For the search it extends, see [Engine internals (v1.2.0)](engine-internals-v.1.2.0.md#8-search). For the speed work before it, see [Engine performance (v1.3.0)](engine-performance-v1.3.0.md).

## The problem: the horizon effect

A search to a fixed depth stops wherever the depth runs out, even in the middle of an exchange. Suppose the last move of a line is a rook taking a defended pawn. The static evaluation counts a pawn won, because the recapture lies one ply beyond the horizon, so the search believes it won material that it will in fact lose.

The engine therefore compared lines whose values could not be compared. Its score swung between odd and even depths depending on who moved last, and it chased captures that only looked good because the reply was not visible.

## The algorithm

At depth 0, `negamax` calls `Search::quiescence` (in `search/alphabeta.rs`) instead of `evaluate`:

```text
quiescence(ply, alpha, beta):
    if is_draw():                       # repetition, fifty moves, insufficient material
        return 0
    if ply ≥ 128:
        return evaluate(board)
    checked ← side to move is in check
    best ← −∞
    if not checked:
        best ← evaluate(board)          # stand pat
        if best ≥ beta:
            return best
        alpha ← max(alpha, best)
    moves ← legal_moves(board)
    if moves is empty:
        return −(MATE − ply) if checked else 0
    if not checked:
        keep only captures and queen promotions
    order moves by MVV-LVA
    for each move:
        make_move; count node; push position hash
        score ← −quiescence(ply + 1, −beta, −alpha)
        pop hash; unmake_move
        if the search must stop: return 0
        best ← max(best, score)
        alpha ← max(alpha, score)
        if alpha ≥ beta: break
    return best
```

### Stand pat

The side to move is never forced to capture. It can usually play a quiet move at least as good as the current position, so the static evaluation is a lower bound on its score. This is called **standing pat**. It has two effects:

- a losing capture cannot lower the score: if Rxd5 exd5 loses the rook, the side simply keeps the static evaluation;
- if the evaluation already reaches `beta`, the opponent will avoid this position and the search returns at once.

Most quiescence nodes end at this test, so moves are generated only after it.

### Which moves are searched

| Position | Moves searched | Why |
| --- | --- | --- |
| Not in check | Captures, en passant and queen promotions | Only these change the material quickly. Lesser promotions are almost never better and would enlarge the tree |
| In check | All legal moves | Standing pat would be wrong, because the side must respond to the check. This also recognizes checkmate at the horizon |

Each capture removes a piece, so captures alone must end. Evasions can follow one another only through a chain of checks, which is limited by repetition detection and the ply limit.

### What the search now sees at the horizon

- **Exchanges** are followed to their end.
- **Checkmate** at the horizon scores as mate rather than by material: at depth 1 the engine finds Qg7# as `mate 1`.
- **Stalemate** scores 0 when the evaluation is below `beta`. A stalemate scoring at least `beta` is cut off by stand pat before moves are generated. A stalemated side is rarely the one ahead, and the normal search still sees it one iteration later, so this is a deliberate trade for speed.
- **Tactics after a check**: at depth 1, Ra8+ followed by Rxh8 is seen as winning the queen, because the evasions and the capture after them are searched.

### What did not change

The principal variation contains only moves of the normal search, so `pv` is still at most `depth` moves long. `seldepth` now reports the real depth including the quiescence search, and `nodes` counts its positions too. Limits are checked in quiescence search exactly as in the normal search.

## Results

Measured on one Windows 11 computer. Speeds are the best of seven bench runs.

| Measurement | 1.3.0 | 1.4.0 |
| --- | ---: | ---: |
| Bench signature (depth 5) | 1,390,581 | 2,492,336 |
| Nodes/second in the bench | 5,797,421 | 2,599,112 |
| Nodes to depth 6, start position | 949,790 | 492,733 |
| Seldepth at depth 6, start position | 6 | 19 |

Speed drops by about half because a quiescence node does more work than the leaf evaluation it replaces. The depth does not drop in the same proportion, because better scores improve pruning: from the start position, depth 6 now takes about half the nodes.

### Stable scores

The score from the start position, by depth:

| Depth | 1 | 2 | 3 | 4 | 5 | 6 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1.3.0 | +50 | 0 | +50 | 0 | +70 | −45 |
| 1.4.0 | +50 | 0 | +50 | 0 | +40 | 0 |

The odd/even swing at low depths remains, because the piece-square tables reward the side that moved last, and no capture is available there to continue the search. What disappears is the large distortion at depth 5 and 6. Version 1.3.0 ended depth 6 on an unanswered capture and chose 1.e3; version 1.4.0 chooses 1.Nc3 with a line that makes sense.

### Examples

After 1.e4 e5 2.Nf3 Nc6 3.Bb5 a6, with `go movetime 1000`:

```text
1.3.0: info depth 7 seldepth 7  score cp 120 … pv b5c4 b7b5 c4b3 g8f6 b1c3 d7d5 c3d5
1.4.0: info depth 6 seldepth 25 score cp 50  … pv b5c6 d7c6 b1c3 c8g4 h2h3 g4f3
```

Version 1.3.0 ends its line on Nxd5, a capture whose recapture it does not see, and its score is inflated. Version 1.4.0 reports one ply less, but it looks 25 plies deep along capture lines, and its +50 is a sober score. It also no longer plans the 4.Bxc6 dxc6 5.Nxe5 pawn grab described in the [known weaknesses of 1.2.0](engine-internals-v.1.2.0.md#12-known-weaknesses).

In the middlegame position `r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10`, the same one-second search gives:

```text
1.3.0: info depth 6 seldepth 6  score cp -110 … pv e2e1 h7h6 c3d5 c5f2 e1f2 f6d5
1.4.0: info depth 6 seldepth 28 score cp 25   … pv c3d5 e7d8 g5f6 g7f6 a1d1 c6d4
```

### Playing strength

A quick local match, without Docker, gave **+43 =3 −2** for 1.4.0 against 1.3.0 in 48 games: 12 openings, each played twice with each color, at 50 ms per move. Games were adjudicated once a score was decisive or stayed near zero for long. This shows the direction only. The result to quote is the SPRT tournament in the arena:

```powershell
./scripts/regress.ps1 -Candidate 1.4.0 -Baseline 1.3.0
```

## Verification

- The tests in `search/alphabeta.rs` call `quiescence` directly: stand pat in a quiet position, declining a losing capture, taking a winning one, following a whole exchange, checkmate, stalemate, answering a check, and the choice of moves.
- The tests in `search/mod.rs` check the effect on the search at depth 1, such as seeing a recapture, a mate and a skewer past the horizon. With quiescence search turned off at the horizon, these tests fail.
- Perft does not change, since move generation does not change.

## What is still missing

Quiescence search has no pruning of its own. Delta pruning (skip a capture that cannot raise the score to alpha even with a safety margin) and static exchange evaluation (skip captures that lose material) would remove many of its nodes. A checking-move search at the first quiescence ply is another usual extension. These are for later versions, when their effect can be measured separately.
