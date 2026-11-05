//! Search entry point and the telemetry every search algorithm reports.
//!
//! [`search`] runs iterative deepening: it searches to depth 1, 2, 3, ...
//! with alpha-beta ([`alphabeta`]) until a limit is reached, and reports
//! every completed depth. [`SearchContext`] counts nodes, enforces the limits
//! and turns reports into the `info` lines the arena stores with each move.

mod alphabeta;
mod heuristics;
mod limits;
mod ordering;
mod report;
pub mod tt;

pub use limits::SearchLimits;
pub use report::{Score, SearchReport};

use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};

use crate::board::Board;
use crate::movegen::{in_check, legal_moves};
use crate::moves::Move;

use alphabeta::{to_uci_score, Search, INFINITY, MATE, MAX_PLY};
use heuristics::Heuristics;
use ordering::{order_moves, Priorities};
use tt::TranspositionTable;

/// Depth searched by a bare `go`, which sets no time, depth or node limit.
const DEFAULT_DEPTH: u32 = 5;
/// How many nodes pass between checks of the clock and the stop flag; a
/// power of two, so the check is a bit mask.
const STOP_CHECK_INTERVAL: u64 = 1024;

/// Counters, limits and the stop signal of one running search.
pub struct SearchContext<'a> {
    started: Instant,
    limits: SearchLimits,
    budget: Option<Duration>,
    stop: Arc<AtomicBool>,
    nodes: u64,
    seldepth: u32,
    aborted: bool,
    /// Limits are ignored until the first iteration completes, so a search
    /// always returns a legal move.
    can_abort: bool,
    on_report: &'a mut dyn FnMut(&SearchReport),
    last_report: Option<SearchReport>,
}

impl<'a> SearchContext<'a> {
    pub fn new(
        board: &Board,
        limits: SearchLimits,
        stop: Arc<AtomicBool>,
        on_report: &'a mut dyn FnMut(&SearchReport),
    ) -> Self {
        Self {
            started: Instant::now(),
            budget: limits.time_budget(board.side_to_move),
            limits,
            stop,
            nodes: 0,
            seldepth: 0,
            aborted: false,
            can_abort: false,
            on_report,
            last_report: None,
        }
    }

    #[cfg(test)]
    pub fn nodes(&self) -> u64 {
        self.nodes
    }

    /// Counts a visited position `ply` half-moves below the root.
    pub fn visit(&mut self, ply: u32) {
        self.nodes += 1;
        self.seldepth = self.seldepth.max(ply);
    }

    /// True once the search must stop: `stop` was received, or the time or
    /// node limit is used up. Cheap enough to call at every node; the clock is
    /// only read every `STOP_CHECK_INTERVAL` nodes.
    pub fn poll_stop(&mut self) -> bool {
        if self.aborted {
            return true;
        }
        if !self.can_abort {
            return false;
        }
        let node_limit = self.limits.nodes.is_some_and(|limit| self.nodes >= limit);
        if node_limit || (self.nodes & (STOP_CHECK_INTERVAL - 1) == 0 && self.time_is_up()) {
            self.aborted = true;
        }
        self.aborted
    }

    fn time_is_up(&self) -> bool {
        self.stop.load(Ordering::Relaxed)
            || self
                .budget
                .is_some_and(|budget| self.started.elapsed() >= budget)
    }

    /// Whether another iteration is worth starting. An iteration usually takes
    /// longer than all previous ones together, so none starts after half of
    /// the time budget.
    fn can_start_iteration(&self) -> bool {
        !self.aborted
            && !self.stop.load(Ordering::Relaxed)
            && self.limits.nodes.is_none_or(|limit| self.nodes < limit)
            && self
                .budget
                .is_none_or(|budget| self.started.elapsed() < budget / 2)
    }

    /// Publishes a completed iteration.
    pub fn report(&mut self, depth: u32, score: Score, pv: Vec<Move>) {
        let report = SearchReport {
            depth,
            seldepth: self.seldepth.max(depth),
            score,
            nodes: self.nodes,
            elapsed: self.started.elapsed(),
            pv,
        };
        (self.on_report)(&report);
        self.last_report = Some(report);
        self.can_abort = true;
    }
}

/// Outcome of a search: the move to play and the final report.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SearchResult {
    /// `None` when the side to move has no legal moves.
    pub best_move: Option<Move>,
    pub report: SearchReport,
}

/// Searches `board` within `limits`, calling `on_report` after every
/// completed depth. `history` holds the position hashes of the game so far,
/// ending with the current position, so repetitions are recognized as draws.
///
/// Killer moves and the history start empty and last for this search only,
/// so a search depends on nothing but its position and the `table`.
pub fn search(
    board: &mut Board,
    history: &[u64],
    limits: SearchLimits,
    stop: Arc<AtomicBool>,
    table: &mut TranspositionTable,
    on_report: &mut dyn FnMut(&SearchReport),
) -> SearchResult {
    let max_depth = match limits.depth {
        Some(depth) => depth.clamp(1, MAX_PLY),
        None if limits.nodes.is_none()
            && !limits.infinite
            && limits.time_budget(board.side_to_move).is_none() =>
        {
            DEFAULT_DEPTH
        }
        None => MAX_PLY,
    };
    let mut context = SearchContext::new(board, limits, stop, on_report);

    let mut root_moves = legal_moves(board);
    if root_moves.is_empty() {
        let score = if in_check(board, board.side_to_move) {
            Score::Mate(0)
        } else {
            Score::Cp(0)
        };
        context.report(0, score, Vec::new());
        let report = context.last_report.take().expect("reported above");
        return SearchResult {
            best_move: None,
            report,
        };
    }

    let mut path = history.to_vec();
    if path.is_empty() {
        path.push(crate::hash::position_hash(board));
    }
    let mut heuristics = Heuristics::new();
    let mut searcher = Search {
        board,
        context: &mut context,
        table,
        heuristics: &mut heuristics,
        path,
    };

    let mut best_line: Vec<Move> = Vec::new();
    for depth in 1..=max_depth {
        if depth > 1 && !searcher.context.can_start_iteration() {
            break;
        }
        let Some((score, line)) = search_root(&mut searcher, &mut root_moves, depth, &best_line)
        else {
            break;
        };
        best_line = line;
        searcher
            .context
            .report(depth, to_uci_score(score), best_line.clone());
        // A forced mate within the horizon will not change with more depth.
        if score.abs() >= MATE - depth as i32 {
            break;
        }
        // With a single legal move there is nothing to decide.
        if root_moves.len() == 1 && searcher.context.budget.is_some() {
            break;
        }
    }

    let report = context
        .last_report
        .take()
        .expect("the first iteration always completes");
    SearchResult {
        best_move: best_line.first().copied(),
        report,
    }
}

/// Searches all root moves to `depth`. Returns the score and principal
/// variation, or `None` when the iteration was aborted. The best move is
/// moved to the front of `root_moves` for the next iteration.
fn search_root(
    searcher: &mut Search,
    root_moves: &mut [Move],
    depth: u32,
    previous_line: &[Move],
) -> Option<(i32, Vec<Move>)> {
    let priorities = Priorities {
        hint: previous_line.first().copied(),
        ..Priorities::default()
    };
    order_moves(searcher.board, root_moves, priorities);

    let mut alpha = -INFINITY;
    let mut best_index = 0;
    let mut best_line = Vec::new();
    let mut child_line = Vec::new();
    for (index, &chess_move) in root_moves.iter().enumerate() {
        let hint = (index == 0 && !previous_line.is_empty()).then(|| &previous_line[1..]);
        let undo = searcher.board.make_move(chess_move);
        searcher.context.visit(1);
        let hash = crate::hash::position_hash(searcher.board);
        searcher.path.push(hash);
        let score = -searcher.negamax(depth - 1, 1, -INFINITY, -alpha, hint, &mut child_line);
        searcher.path.pop();
        searcher.board.unmake_move(chess_move, undo);

        if searcher.context.poll_stop() {
            return None;
        }
        if score > alpha {
            alpha = score;
            best_index = index;
            best_line.clear();
            best_line.push(chess_move);
            best_line.extend_from_slice(&child_line);
        }
    }
    root_moves[..=best_index].rotate_right(1);
    Some((alpha, best_line))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fen::parse_fen;

    fn run(fen: &str, limits: SearchLimits) -> (SearchResult, Vec<SearchReport>) {
        let mut board = parse_fen(fen).unwrap();
        let history = vec![crate::hash::position_hash(&board)];
        let mut reports = Vec::new();
        let result = search(
            &mut board,
            &history,
            limits,
            Arc::new(AtomicBool::new(false)),
            &mut TranspositionTable::new(1),
            &mut |report| reports.push(report.clone()),
        );
        (result, reports)
    }

    fn depth(depth: u32) -> SearchLimits {
        SearchLimits {
            depth: Some(depth),
            ..SearchLimits::default()
        }
    }

    fn best(result: &SearchResult) -> String {
        result.best_move.unwrap().to_uci()
    }

    #[test]
    fn reports_every_depth() {
        let (result, reports) = run(
            "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
            depth(3),
        );
        let depths: Vec<u32> = reports.iter().map(|report| report.depth).collect();
        assert_eq!(depths, vec![1, 2, 3]);
        assert_eq!(reports[0].nodes, 20);
        assert!(reports.windows(2).all(|pair| pair[0].nodes < pair[1].nodes));
        assert_eq!(&result.report, reports.last().unwrap());
        assert_eq!(result.report.pv.len(), 3);
        assert_eq!(result.report.pv[0], result.best_move.unwrap());
    }

    #[test]
    fn finds_mate_in_one() {
        let (result, _) = run("7k/5ppp/8/8/8/8/8/R5K1 w - - 0 1", depth(3));
        assert_eq!(best(&result), "a1a8");
        assert_eq!(result.report.score, Score::Mate(1));
    }

    #[test]
    fn finds_mate_in_two() {
        // 1. Kb6 Kb8 2. Rh8#
        let (result, _) = run("k7/8/2K5/8/8/8/8/7R w - - 0 1", depth(4));
        assert_eq!(result.report.score, Score::Mate(2));
        assert_eq!(result.report.pv.len(), 3);
    }

    #[test]
    fn reports_being_mated() {
        // Black's only move is Kb8, then Rh8#.
        let (result, _) = run("k7/8/1K6/8/8/8/8/7R b - - 0 1", depth(3));
        assert_eq!(result.report.score, Score::Mate(-1));
        assert_eq!(best(&result), "a8b8");
    }

    #[test]
    fn captures_hanging_queen() {
        let (result, _) = run("4k3/8/8/3q4/8/8/8/3RK3 w - - 0 1", depth(2));
        assert_eq!(best(&result), "d1d5");
        assert!(matches!(result.report.score, Score::Cp(cp) if cp > 300));
    }

    #[test]
    fn avoids_losing_the_queen() {
        // Qxd7?? loses the queen to the king; the queen should stay safe.
        let (result, _) = run("4k3/3p4/8/8/8/8/8/3QK3 w - - 0 1", depth(3));
        assert_ne!(best(&result), "d1d7");
    }

    #[test]
    fn prefers_mate_to_stalemate() {
        // Qg7# mates; Qg6 would stalemate the king on h8.
        let (result, _) = run("7k/8/5K2/8/8/8/8/6Q1 w - - 0 1", depth(2));
        assert_eq!(best(&result), "g1g7");
        assert_eq!(result.report.score, Score::Mate(1));
    }

    #[test]
    fn scores_repetition_as_draw() {
        // White is a rook down but can repeat: the knight shuffle returns to a
        // position already seen twice.
        let mut board = parse_fen("4k3/8/8/8/8/8/1r6/4K1N1 w - - 0 1").unwrap();
        let start = crate::hash::position_hash(&board);
        let mut history = vec![start];
        for notation in ["g1f3", "e8d8", "f3g1", "d8e8"] {
            let chess_move = legal_moves(&mut board)
                .into_iter()
                .find(|m| m.to_uci() == notation)
                .unwrap();
            board.make_move(chess_move);
            history.push(crate::hash::position_hash(&board));
        }
        let mut reports = Vec::new();
        let result = search(
            &mut board,
            &history,
            depth(1),
            Arc::new(AtomicBool::new(false)),
            &mut TranspositionTable::new(1),
            &mut |report| reports.push(report.clone()),
        );
        // Ng1-f3 repeats the position after White's first move: a draw beats
        // being a rook down.
        assert_eq!(best(&result), "g1f3");
        assert_eq!(result.report.score, Score::Cp(0));
    }

    #[test]
    fn sees_the_recapture_beyond_the_horizon() {
        // Rxd5 wins a pawn but exd5 recaptures one ply past depth 1; without
        // quiescence search the rook would look like it won a pawn for free.
        let (result, _) = run("4k3/8/4p3/3p4/8/8/8/3RK3 w - - 0 1", depth(1));
        assert_ne!(best(&result), "d1d5");
    }

    #[test]
    fn captures_when_the_exchange_wins() {
        // The queen on d5 is defended by a pawn, but pawn takes queen wins it.
        let (result, _) = run("4k3/8/4p3/3q4/4P3/8/8/4K3 w - - 0 1", depth(1));
        assert_eq!(best(&result), "e4d5");
    }

    #[test]
    fn sees_checkmate_at_the_horizon() {
        // After Qg7 at depth 1 Black has no moves: it must be scored as mate,
        // not by the material on the board.
        let (result, _) = run("7k/8/5K2/8/8/8/8/6Q1 w - - 0 1", depth(1));
        assert_eq!(best(&result), "g1g7");
        assert_eq!(result.report.score, Score::Mate(1));
    }

    #[test]
    fn sees_a_skewer_beyond_the_horizon() {
        // Ra8+ forces the king off the rank, and Rxh8 then wins the queen: a
        // rook against nothing instead of a rook against a queen. At depth 1
        // this needs the evasions and the capture after them.
        let (result, reports) = run("4k2q/8/8/8/8/8/8/R3K3 w - - 0 1", depth(1));
        assert_eq!(best(&result), "a1a8");
        assert!(matches!(result.report.score, Score::Cp(cp) if cp > 300));
        assert!(reports[0].seldepth >= 3);
    }

    /// Searches `fen` twice with one table kept between the two searches.
    fn run_twice(fen: &str, depth_limit: u32) -> (SearchResult, SearchResult) {
        let mut table = TranspositionTable::new(1);
        let mut once = || {
            let mut board = parse_fen(fen).unwrap();
            let history = vec![crate::hash::position_hash(&board)];
            search(
                &mut board,
                &history,
                depth(depth_limit),
                Arc::new(AtomicBool::new(false)),
                &mut table,
                &mut |_| {},
            )
        };
        (once(), once())
    }

    #[test]
    fn table_saves_nodes_when_a_position_is_searched_again() {
        let (first, second) = run_twice(
            "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
            4,
        );
        assert!(
            second.report.nodes < first.report.nodes / 2,
            "{} then {} nodes",
            first.report.nodes,
            second.report.nodes
        );
    }

    #[test]
    fn stored_results_keep_the_principal_variation_complete() {
        let (_, second) = run_twice(
            "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
            5,
        );
        assert_eq!(second.report.pv.len(), 5);
        assert_eq!(second.report.pv[0], second.best_move.unwrap());
    }

    #[test]
    fn stored_mate_scores_stay_correct() {
        // 1. Kb6 Kb8 2. Rh8#
        let (first, second) = run_twice("k7/8/2K5/8/8/8/8/7R w - - 0 1", 4);
        assert_eq!(first.report.score, Score::Mate(2));
        assert_eq!(second.report.score, Score::Mate(2));
        assert_eq!(second.best_move, first.best_move);
    }

    #[test]
    fn reports_terminal_positions() {
        let (mated, _) = run(
            "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq - 1 3",
            SearchLimits::default(),
        );
        assert_eq!(mated.best_move, None);
        assert_eq!(mated.report.score, Score::Mate(0));
        assert_eq!(mated.report.depth, 0);

        let (stalemate, _) = run("7k/5Q2/6K1/8/8/8/8/8 b - - 0 1", SearchLimits::default());
        assert_eq!(stalemate.best_move, None);
        assert_eq!(stalemate.report.score, Score::Cp(0));
    }

    #[test]
    fn node_limit_stops_after_the_first_depth() {
        let (result, reports) = run(
            "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
            SearchLimits {
                nodes: Some(5),
                ..SearchLimits::default()
            },
        );
        // Depth 1 always completes (20 nodes); no deeper iteration starts.
        assert_eq!(reports.len(), 1);
        assert_eq!(result.report.nodes, 20);
        assert!(result.best_move.is_some());
    }

    #[test]
    fn time_limit_is_respected() {
        let started = Instant::now();
        let (result, _) = run(
            "r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3",
            SearchLimits {
                movetime: Some(Duration::from_millis(200)),
                ..SearchLimits::default()
            },
        );
        assert!(result.best_move.is_some());
        assert!(started.elapsed() < Duration::from_millis(400));
    }

    #[test]
    fn stop_flag_ends_an_infinite_search() {
        let mut board =
            parse_fen("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1").unwrap();
        let history = vec![crate::hash::position_hash(&board)];
        let stop = Arc::new(AtomicBool::new(false));
        let setter = Arc::clone(&stop);
        let handle = std::thread::spawn(move || {
            std::thread::sleep(Duration::from_millis(100));
            setter.store(true, Ordering::Relaxed);
        });
        let limits = SearchLimits {
            infinite: true,
            ..SearchLimits::default()
        };
        let mut table = TranspositionTable::new(1);
        let result = search(&mut board, &history, limits, stop, &mut table, &mut |_| {});
        handle.join().unwrap();
        assert!(result.best_move.is_some());
    }
}
