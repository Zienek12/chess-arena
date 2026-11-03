# Analysis and Statistics

The arena judges engines in two ways: by results (who wins, and how sure we can be) and by move quality (how close each move was to Stockfish's choice).

## Results

### Elo, error and LOS

For every tournament, `GET /api/tournaments/{id}` returns `stats` from engine A's point of view (package `stats`):

| Field | Meaning |
| --- | --- |
| `score` | Points per game, draws counting half. |
| `elo` | Logistic Elo difference: `-400 · log10(1/score - 1)`. Omitted while every game was won or lost (the estimate is infinite). |
| `elo_error` | Half of the 95% confidence interval, from the trinomial (win/draw/loss) variance of the score. |
| `los` | Likelihood of superiority: the probability that engine A is stronger, `0.5 · (1 + erf((W - L) / sqrt(2 · (W + L))))`. Draws carry no information here. |

### SPRT

A tournament can carry a sequential probability ratio test (`"sprt": {"elo0", "elo1", "alpha", "beta"}`), as used by Fishtest to test engine changes. It compares H0 (engine A is about `elo0` stronger) with H1 (about `elo1` stronger). After every finished game the arena computes the log-likelihood ratio with the generalized SPRT approximation:

```text
LLR = N · (s1 - s0) · (2·x̄ - s0 - s1) / (2·σ²)
```

where `s0` and `s1` are the expected scores for `elo0` and `elo1`, `x̄` the mean score and `σ²` its per-game variance. The test accepts H1 when the LLR reaches `ln((1 - β) / α)` and H0 when it falls to `ln(β / (1 - α))`. Half a game is added to each of wins, draws and losses before computing the LLR, so a one-sided run (all wins) has a variance and can be decided. With hundreds of games the effect is negligible.

When the test decides, the same transaction that stored the game result sets `sprt_result` (`H1` passed, `H0` failed), completes the tournament and cancels the games that have not been played. `game_count` is then only an upper bound.

Typical settings: `elo0 = 0`, `elo1 = 10`, α = β = 0.05 to detect small improvements (thousands of games); a wider gap such as `elo1 = 50` decides in a few hundred games while the engine is still improving quickly.

### Leaderboard

`GET /api/leaderboard` rates every engine that finished a game, across all tournaments. Ratings come from the Bradley-Terry model (draws count as half a win), fitted with minorization-maximization iterations. Each engine also plays two virtual draws against an average player. This keeps ratings finite for engines that won or lost every game and pulls engines with few games towards the average. Ratings are relative: they average to 0. Engines that never met are compared through common opponents.

### Absolute Elo from anchors

Relative ratings show how far apart engines are, not how strong they are. Stockfish limited with the UCI options `UCI_LimitStrength=true` and `UCI_Elo=N` declares its strength, so it can serve as an **anchor**. When anchors have played, the leaderboard shifts every rating by the average difference between the anchors' declared Elo and their fitted ratings:

```text
offset = mean over anchors (declared_elo − rating)
elo    = rating + offset
```

The anchors therefore average their declared Elo, and every other engine is placed on that scale. Several anchors around the engine's strength (for example 1500, 1800 and 2100) make the estimate robust; with one anchor it is only as good as the games against it.

Treat the result as an estimate. Stockfish calibrates `UCI_Elo` against other engines at longer time controls (around two minutes per game) and in its own playing style. Its limited mode chooses moves from a shallow search with random noise. At one second per move, and against a different engine, the scale can be off by about 100 Elo. It is consistent between versions of our engine, which is what the comparison needs.

To calibrate a version, register Stockfish at several levels (name `stockfish`, version `17.1-elo1800`, image `chess-arena-stockfish:local`, UCI options `UCI_LimitStrength=true, UCI_Elo=1800`) and play about 50 games against each on the opening suite.

## Move quality

Package `analysis` reviews every finished game with a reference engine, Stockfish by default.

1. A worker leases the most recently finished game without analysis (`game_analyses`, the same lease pattern as games). The engine keeps running between games and stops when the queue is empty.
2. Stockfish evaluates every position of the game with a fixed node budget (`ANALYSIS_NODES`, 200,000 by default, depth about 18). A final checkmate or draw on the board needs no search.
3. Each move is judged from the evaluation before it (the position the mover faced, with Stockfish's best move) and after it:

| Field | Meaning |
| --- | --- |
| `cp_loss` | Centipawns lost against the best move, from the mover's view; each evaluation is clamped to ±1000 first, so one blunder in a lost position does not dominate averages. |
| `accuracy` | 0–100 from the drop in winning chances, using the Lichess formulas: win% = `50 + 50 · (2 / (1 + e^(-0.00368208·cp)) - 1)`, accuracy = `103.1668 · e^(-0.04354 · drop) - 3.1669 + 1`. |
| `classification` | `best` when the move is Stockfish's choice, otherwise `inaccuracy`, `mistake` or `blunder` for a drop in winning chances of at least 5, 10 or 15 percentage points, else `good`. |

Evaluations are stored from White's point of view. A mate is stored as `±10000` centipawns plus `mate` (moves to mate; `0` means checkmate on the board).

Per-move verdicts appear in `GET /api/games/{id}` (`moves[].analysis`), and per-engine aggregates (accuracy, average centipawn loss, inaccuracies, mistakes, blunders) in the tournament telemetry and the leaderboard.

If the analysis engine cannot start (for example, its image is missing), the job goes back to the queue without counting an attempt, and the worker waits a minute before trying again.

## Stockfish as an opponent

`engines/stockfish/Dockerfile` packages Debian's Stockfish (17.1, with its NNUE network). `docker compose up --build` builds it as `chess-arena-stockfish:local`. Register it like any engine; a weaker sparring partner uses UCI options such as `UCI_LimitStrength=true, UCI_Elo=1500` (Stockfish accepts 1320–3190) or `Skill Level=5`.

## Regression testing

`scripts/regress.ps1` tests a new engine version against an older one in the local arena:

```powershell
./scripts/regress.ps1 -Candidate 1.3.0                     # vs the newest other registered version
./scripts/regress.ps1 -Candidate 1.3.0 -Baseline 1.2.0 -Elo1 50
./scripts/regress.ps1 -Candidate dev -CandidateImage chess-arena-engine:local -Baseline 1.2.0
```

It registers both images if needed, starts an SPRT tournament on the "Balanced openings (50)" suite (default H0 = 0, H1 = 20 Elo, at most 2000 games, 1000 ms per move), prints progress, and exits with 0 when the candidate passed, 1 when it failed and 2 without a decision. Run `Get-Help ./scripts/regress.ps1 -Full` for every option.
