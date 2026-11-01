//! Search entry point and the telemetry every search algorithm reports.
//!
//! A search algorithm plugs into [`SearchContext`]: it calls
//! [`SearchContext::visit`] for every position it enters, checks
//! [`SearchContext::should_stop`] regularly, and calls
//! [`SearchContext::report`] after every completed iteration. The context
//! turns that into the `info` lines the arena stores with each move.

mod limits;
mod report;

pub use limits::SearchLimits;
pub use report::{Score, SearchReport};

use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};

use rand::seq::SliceRandom;

use crate::board::Board;
use crate::movegen::{in_check, legal_moves};
use crate::moves::Move;

/// Counters, limits and the stop signal of one running search.
pub struct SearchContext<'a> {
    started: Instant,
    limits: SearchLimits,
    budget: Option<Duration>,
    stop: Arc<AtomicBool>,
    nodes: u64,
    seldepth: u32,
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
            on_report,
            last_report: None,
        }
    }

    /// Counts a visited position `ply` half-moves below the root.
    pub fn visit(&mut self, ply: u32) {
        self.nodes += 1;
        self.seldepth = self.seldepth.max(ply);
    }

    /// True once `stop` was received or the time or node limit is used up.
    pub fn should_stop(&self) -> bool {
        self.stop.load(Ordering::Relaxed)
            || self
                .budget
                .is_some_and(|budget| self.started.elapsed() >= budget)
            || self.limits.nodes.is_some_and(|limit| self.nodes >= limit)
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
/// completed iteration.
pub fn search(
    board: &mut Board,
    limits: SearchLimits,
    stop: Arc<AtomicBool>,
    on_report: &mut dyn FnMut(&SearchReport),
) -> SearchResult {
    let mut context = SearchContext::new(board, limits, stop, on_report);
    let best_move = random_root_move(board, &mut context);
    let report = context
        .last_report
        .take()
        .expect("a search reports at least once");
    SearchResult { best_move, report }
}

/// Placeholder until alpha-beta exists: picks a random legal move, but
/// reports real telemetry. Every root move is visited, and a move that mates
/// at once is reported as `mate 1`.
fn random_root_move(board: &mut Board, context: &mut SearchContext) -> Option<Move> {
    let moves = legal_moves(board);
    if moves.is_empty() {
        let score = if in_check(board, board.side_to_move) {
            Score::Mate(0)
        } else {
            Score::Cp(0)
        };
        context.report(0, score, Vec::new());
        return None;
    }

    let visited = visit_root_moves(board, &moves, context);
    let &(chess_move, mates) = visited
        .choose(&mut rand::thread_rng())
        .expect("at least one move was visited");
    let score = if mates { Score::Mate(1) } else { Score::Cp(0) };
    context.report(1, score, vec![chess_move]);
    Some(chess_move)
}

/// Visits each root move until the search must stop, recording whether the
/// move checkmates. The first move is always visited, so a move exists even
/// on a tiny budget.
fn visit_root_moves(
    board: &mut Board,
    moves: &[Move],
    context: &mut SearchContext,
) -> Vec<(Move, bool)> {
    let mut visited = Vec::with_capacity(moves.len());
    for &chess_move in moves {
        if !visited.is_empty() && context.should_stop() {
            break;
        }
        let undo = board.make_move(chess_move);
        context.visit(1);
        let mates = legal_moves(board).is_empty() && in_check(board, board.side_to_move);
        board.unmake_move(chess_move, undo);
        visited.push((chess_move, mates));
    }
    visited
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fen::parse_fen;

    fn run(fen: &str, limits: SearchLimits) -> (SearchResult, Vec<SearchReport>) {
        let mut board = parse_fen(fen).unwrap();
        let mut reports = Vec::new();
        let result = search(
            &mut board,
            limits,
            Arc::new(AtomicBool::new(false)),
            &mut |report| reports.push(report.clone()),
        );
        (result, reports)
    }

    #[test]
    fn reports_root_telemetry() {
        let (result, reports) = run(
            "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
            SearchLimits::default(),
        );
        assert_eq!(reports.len(), 1);
        let report = &result.report;
        assert_eq!(report, &reports[0]);
        assert_eq!((report.depth, report.seldepth, report.nodes), (1, 1, 20));
        assert_eq!(report.score, Score::Cp(0));
        assert_eq!(report.pv, vec![result.best_move.unwrap()]);
    }

    #[test]
    fn detects_mating_root_moves() {
        // Back-rank mate: only Ra8 checkmates.
        let mut board = parse_fen("7k/5ppp/8/8/8/8/8/R5K1 w - - 0 1").unwrap();
        let moves = legal_moves(&mut board);
        let mut ignore = |_: &SearchReport| {};
        let mut context = SearchContext::new(
            &board,
            SearchLimits::default(),
            Arc::new(AtomicBool::new(false)),
            &mut ignore,
        );
        let mating: Vec<String> = visit_root_moves(&mut board, &moves, &mut context)
            .into_iter()
            .filter(|&(_, mates)| mates)
            .map(|(chess_move, _)| chess_move.to_uci())
            .collect();
        assert_eq!(mating, vec!["a1a8"]);
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
    fn stops_at_node_limit_but_still_returns_a_move() {
        let (result, _) = run(
            "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
            SearchLimits {
                nodes: Some(5),
                ..SearchLimits::default()
            },
        );
        assert_eq!(result.report.nodes, 5);
        assert!(result.best_move.is_some());
    }
}
