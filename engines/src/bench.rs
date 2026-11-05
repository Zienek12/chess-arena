//! A fixed benchmark: every position below is searched to the same depth.
//!
//! The total node count is the engine's *bench signature*. It changes only
//! when the search or evaluation behaves differently, so a change meant only
//! to make the engine faster must keep it, and CI compares it with
//! `engines/bench-signature.txt`. Nodes per second measure speed.

use std::sync::atomic::AtomicBool;
use std::sync::Arc;
use std::time::{Duration, Instant};

use crate::fen::parse_fen;
use crate::hash::position_hash;
use crate::search::tt::{TranspositionTable, DEFAULT_SIZE_MIB};
use crate::search::{search, SearchLimits};

/// Depth searched when `bench` gets no depth argument.
pub const DEFAULT_DEPTH: u32 = 5;

/// Openings, middlegames full of tactics, and endgames. The first six are the
/// perft reference positions.
const POSITIONS: [&str; 18] = [
    "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
    "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
    "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
    "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
    "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
    "r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10",
    "r1bqkbnr/pppp1ppp/2n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R b KQkq - 3 3",
    "rnbqkb1r/pp2pppp/3p1n2/8/3NP3/8/PPP2PPP/RNBQKB1R w KQkq - 1 5",
    "r1bq1rk1/pp2b1pp/n1pp1n2/3P1p2/2P1p3/2N1P2N/PP2BPPP/R1BQ1RK1 b - - 2 10",
    "4rrk1/2p1b1p1/p1p3q1/4p3/2P2n1p/1P1NR2P/PB3PP1/3R1QK1 b - - 2 24",
    "r3qbrk/6p1/2b2pPp/p3pP1Q/PpPpP2P/3P1B2/2PB3K/R5R1 w - - 16 42",
    "6k1/1R3p2/6p1/2Bp3p/3P2q1/P7/1P2rQ1K/5R2 b - - 4 44",
    "7r/2p3k1/1p1p1qp1/1P1Bp3/p1P2r1P/P7/4R3/Q4RK1 w - - 0 36",
    "2r4r/1p4k1/1Pnp4/3Qb1pq/8/4BpPp/5P2/2RR1BK1 w - - 0 42",
    "8/8/1p2k1p1/3p3p/1p1P1P1P/1P2PK2/8/8 w - - 3 54",
    "8/8/8/8/5kp1/P7/8/1K1N4 w - - 0 1",
    "8/8/3P3k/8/1p6/8/1P6/1K3n2 b - - 0 1",
    "6k1/5ppp/8/8/8/8/5PPP/3R2K1 w - - 0 1",
];

/// Totals of one benchmark run.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct BenchResult {
    pub positions: usize,
    pub depth: u32,
    pub nodes: u64,
    pub elapsed: Duration,
}

impl BenchResult {
    pub fn nps(&self) -> u64 {
        let micros = self.elapsed.as_micros().max(1);
        (u128::from(self.nodes) * 1_000_000 / micros) as u64
    }

    /// The summary printed by the `bench` command. "Nodes searched" is the
    /// signature that tools read.
    pub fn lines(&self) -> Vec<String> {
        vec![
            format!("Positions searched: {}", self.positions),
            format!("Depth: {}", self.depth),
            format!("Nodes searched: {}", self.nodes),
            format!("Time (ms): {}", self.elapsed.as_millis()),
            format!("Nodes/second: {}", self.nps()),
        ]
    }
}

/// Searches every benchmark position to `depth`, each as a fresh game.
pub fn run(depth: u32) -> BenchResult {
    let started = Instant::now();
    let mut nodes = 0;
    for fen in POSITIONS {
        let mut board = parse_fen(fen).expect("benchmark positions are valid");
        let history = [position_hash(&board)];
        let limits = SearchLimits {
            depth: Some(depth),
            ..SearchLimits::default()
        };
        let result = search(
            &mut board,
            &history,
            limits,
            Arc::new(AtomicBool::new(false)),
            // A new table of a fixed size for every position, so that the node
            // count depends on nothing but the search.
            &mut TranspositionTable::new(DEFAULT_SIZE_MIB),
            &mut |_| {},
        );
        nodes += result.report.nodes;
    }
    BenchResult {
        positions: POSITIONS.len(),
        depth,
        nodes,
        elapsed: started.elapsed(),
    }
}

/// Parses the optional depth argument of `bench`.
pub fn parse_depth(argument: Option<&str>) -> Result<u32, String> {
    match argument {
        None => Ok(DEFAULT_DEPTH),
        Some(text) => match text.parse::<u32>() {
            Ok(depth) if depth >= 1 => Ok(depth),
            _ => Err(format!("invalid bench depth {}", text)),
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::movegen::legal_moves;

    #[test]
    fn positions_are_valid_and_playable() {
        for fen in POSITIONS {
            let mut board = parse_fen(fen).unwrap_or_else(|error| panic!("{fen}: {error}"));
            assert!(!legal_moves(&mut board).is_empty(), "{fen} has no moves");
        }
    }

    #[test]
    fn bench_is_deterministic() {
        let first = run(2);
        let second = run(2);
        assert_eq!(first.nodes, second.nodes);
        assert_eq!(first.positions, POSITIONS.len());
        assert!(first.lines()[2].starts_with("Nodes searched: "));
    }

    #[test]
    fn parses_depth() {
        assert_eq!(parse_depth(None), Ok(DEFAULT_DEPTH));
        assert_eq!(parse_depth(Some("3")), Ok(3));
        assert!(parse_depth(Some("0")).is_err());
        assert!(parse_depth(Some("deep")).is_err());
    }
}
