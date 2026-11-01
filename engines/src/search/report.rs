use std::fmt;
use std::time::Duration;

use crate::moves::Move;

/// Evaluation from the point of view of the side to move.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Score {
    /// Centipawns.
    Cp(i32),
    /// Mate in this many moves; negative when the side to move is mated.
    /// `Mate(0)` means the side to move is already checkmated.
    Mate(i32),
}

impl fmt::Display for Score {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Score::Cp(centipawns) => write!(f, "cp {}", centipawns),
            Score::Mate(moves) => write!(f, "mate {}", moves),
        }
    }
}

/// Telemetry of one completed search iteration. The arena stores the last
/// report before `bestmove` with every move.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct SearchReport {
    /// Completed iteration depth in plies.
    pub depth: u32,
    /// Deepest ply reached, including extensions.
    pub seldepth: u32,
    pub score: Score,
    /// Positions visited since the search started.
    pub nodes: u64,
    pub elapsed: Duration,
    /// Principal variation, starting with the best move.
    pub pv: Vec<Move>,
}

impl SearchReport {
    pub fn nps(&self) -> u64 {
        let micros = self.elapsed.as_micros().max(1);
        (u128::from(self.nodes) * 1_000_000 / micros) as u64
    }

    /// Formats the report as a UCI `info` line.
    pub fn to_info_line(&self) -> String {
        let mut line = format!(
            "info depth {} seldepth {} score {} nodes {} nps {} time {}",
            self.depth,
            self.seldepth,
            self.score,
            self.nodes,
            self.nps(),
            self.elapsed.as_millis()
        );
        if !self.pv.is_empty() {
            let pv: Vec<String> = self
                .pv
                .iter()
                .map(|chess_move| chess_move.to_uci())
                .collect();
            line.push_str(" pv ");
            line.push_str(&pv.join(" "));
        }
        line
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::Square;
    use crate::moves::MoveKind;

    #[test]
    fn formats_info_line() {
        let report = SearchReport {
            depth: 2,
            seldepth: 3,
            score: Score::Cp(-35),
            nodes: 500,
            elapsed: Duration::from_millis(250),
            pv: vec![
                Move::new(Square::E2, Square::E4, MoveKind::DoublePawnPush),
                Move::new(Square::E7, Square::E5, MoveKind::DoublePawnPush),
            ],
        };
        assert_eq!(
            report.to_info_line(),
            "info depth 2 seldepth 3 score cp -35 nodes 500 nps 2000 time 250 pv e2e4 e7e5"
        );
    }

    #[test]
    fn omits_empty_pv() {
        let report = SearchReport {
            depth: 0,
            seldepth: 0,
            score: Score::Mate(0),
            nodes: 0,
            elapsed: Duration::ZERO,
            pv: Vec::new(),
        };
        assert_eq!(
            report.to_info_line(),
            "info depth 0 seldepth 0 score mate 0 nodes 0 nps 0 time 0"
        );
    }
}
