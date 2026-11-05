//! Learned ordering of quiet moves.
//!
//! A quiet move gives no static hint that it is good, unlike a capture. But a
//! move that refuted one position often refutes similar ones too, so the search
//! remembers which quiet moves caused beta cutoffs:
//!
//! - **Killer moves** are the last two quiet cutoff moves at each ply, which
//!   tend to work in the sibling positions searched next.
//! - The **history heuristic** scores every move by side, source and target
//!   square, for all plies together. It rewards a cutoff move and penalizes the
//!   quiet moves tried before it, more so the deeper the search was.

use crate::board::Color;
use crate::moves::Move;

use super::alphabeta::MAX_PLY;

/// History scores stay within plus or minus this value.
const MAX_HISTORY: i32 = 16_384;
/// Both killer scores are above every history score and, as the tests check,
/// below the promotions and captures of `ordering.rs`.
const FIRST_KILLER_SCORE: i32 = 80_000;
const SECOND_KILLER_SCORE: i32 = 79_000;

pub struct Heuristics {
    killers: [[Option<Move>; 2]; MAX_PLY as usize],
    history: [[[i32; 64]; 64]; 2],
}

impl Heuristics {
    pub fn new() -> Self {
        Self {
            killers: [[None; 2]; MAX_PLY as usize],
            history: [[[0; 64]; 64]; 2],
        }
    }

    /// The ordering score of a quiet move at `ply`.
    pub fn quiet_score(&self, side: Color, chess_move: Move, ply: u32) -> i32 {
        let [first, second] = self.killers[ply as usize];
        if first == Some(chess_move) {
            FIRST_KILLER_SCORE
        } else if second == Some(chess_move) {
            SECOND_KILLER_SCORE
        } else {
            self.history[side.index()][from(chess_move)][to(chess_move)]
        }
    }

    /// Learns from a beta cutoff by the quiet move `cutoff`, after searching
    /// the moves `tried` before it without a cutoff.
    pub fn record_cutoff(
        &mut self,
        side: Color,
        ply: u32,
        depth: u32,
        cutoff: Move,
        tried: &[Move],
    ) {
        let killers = &mut self.killers[ply as usize];
        if killers[0] != Some(cutoff) {
            killers[1] = killers[0];
            killers[0] = Some(cutoff);
        }

        let bonus = (depth * depth).min(MAX_HISTORY as u32) as i32;
        self.update_history(side, cutoff, bonus);
        for &chess_move in tried {
            if chess_move.is_quiet() {
                self.update_history(side, chess_move, -bonus);
            }
        }
    }

    /// Moves a score towards the limit by `change`, in smaller steps the nearer
    /// it already is. The score cannot leave the limits, and recent results
    /// weigh more than old ones.
    fn update_history(&mut self, side: Color, chess_move: Move, change: i32) {
        let score = &mut self.history[side.index()][from(chess_move)][to(chess_move)];
        *score += change - *score * change.abs() / MAX_HISTORY;
    }
}

fn from(chess_move: Move) -> usize {
    chess_move.from.index() as usize
}

fn to(chess_move: Move) -> usize {
    chess_move.to.index() as usize
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::Square;
    use crate::moves::MoveKind;

    const SIDE: Color = Color::White;

    fn quiet(from: Square, to: Square) -> Move {
        Move::new(from, to, MoveKind::Quiet)
    }

    fn a() -> Move {
        quiet(Square::A1, Square::A2)
    }

    fn b() -> Move {
        quiet(Square::B1, Square::B2)
    }

    fn c() -> Move {
        quiet(Square::C1, Square::C2)
    }

    fn history_score(heuristics: &Heuristics, side: Color, chess_move: Move) -> i32 {
        heuristics.history[side.index()][from(chess_move)][to(chess_move)]
    }

    #[test]
    fn unknown_moves_score_zero() {
        assert_eq!(Heuristics::new().quiet_score(SIDE, a(), 0), 0);
    }

    #[test]
    fn latest_two_cutoff_moves_are_the_killers_of_their_ply() {
        let mut heuristics = Heuristics::new();
        heuristics.record_cutoff(SIDE, 3, 1, a(), &[]);
        heuristics.record_cutoff(SIDE, 3, 1, b(), &[]);
        assert_eq!(heuristics.quiet_score(SIDE, b(), 3), FIRST_KILLER_SCORE);
        assert_eq!(heuristics.quiet_score(SIDE, a(), 3), SECOND_KILLER_SCORE);

        heuristics.record_cutoff(SIDE, 3, 1, c(), &[]);
        assert_eq!(heuristics.quiet_score(SIDE, c(), 3), FIRST_KILLER_SCORE);
        assert_eq!(heuristics.quiet_score(SIDE, b(), 3), SECOND_KILLER_SCORE);
        assert!(heuristics.quiet_score(SIDE, a(), 3) < SECOND_KILLER_SCORE);
    }

    #[test]
    fn repeated_killer_does_not_push_out_the_other() {
        let mut heuristics = Heuristics::new();
        heuristics.record_cutoff(SIDE, 0, 1, a(), &[]);
        heuristics.record_cutoff(SIDE, 0, 1, b(), &[]);
        heuristics.record_cutoff(SIDE, 0, 1, b(), &[]);
        assert_eq!(heuristics.quiet_score(SIDE, a(), 0), SECOND_KILLER_SCORE);
    }

    #[test]
    fn killers_belong_to_one_ply() {
        let mut heuristics = Heuristics::new();
        heuristics.record_cutoff(SIDE, 3, 2, a(), &[]);
        assert_eq!(heuristics.quiet_score(SIDE, a(), 4), 4);
    }

    #[test]
    fn history_rewards_the_cutoff_and_penalizes_the_quiet_moves_before_it() {
        let mut heuristics = Heuristics::new();
        heuristics.record_cutoff(SIDE, 0, 4, a(), &[b()]);
        assert_eq!(history_score(&heuristics, SIDE, a()), 16);
        assert_eq!(history_score(&heuristics, SIDE, b()), -16);
    }

    #[test]
    fn history_ignores_captures_that_were_tried() {
        let capture = Move::new(Square::C1, Square::C2, MoveKind::Capture);
        let mut heuristics = Heuristics::new();
        heuristics.record_cutoff(SIDE, 0, 4, a(), &[capture]);
        assert_eq!(history_score(&heuristics, SIDE, capture), 0);
    }

    #[test]
    fn history_is_kept_per_side() {
        let mut heuristics = Heuristics::new();
        heuristics.record_cutoff(Color::White, 0, 4, a(), &[]);
        assert_eq!(history_score(&heuristics, Color::Black, a()), 0);
    }

    #[test]
    fn history_stays_within_its_limits() {
        let mut heuristics = Heuristics::new();
        for _ in 0..10_000 {
            heuristics.record_cutoff(SIDE, 0, MAX_PLY, a(), &[b()]);
            assert!(history_score(&heuristics, SIDE, a()) <= MAX_HISTORY);
            assert!(history_score(&heuristics, SIDE, b()) >= -MAX_HISTORY);
        }
        const { assert!(MAX_HISTORY < SECOND_KILLER_SCORE) };
    }
}
