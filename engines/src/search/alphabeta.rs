//! Negamax search with alpha-beta pruning.
//!
//! Scores are in centipawns from the side to move. Mate scores encode the
//! distance to mate, so the search prefers the fastest mate and the slowest
//! defeat: being mated `ply` half-moves below the root scores `-(MATE - ply)`.

use crate::board::Board;
use crate::eval::evaluate;
use crate::hash::position_hash;
use crate::movegen::{in_check, legal_moves};
use crate::moves::Move;
use crate::status::insufficient_material;

use super::ordering::order_moves;
use super::{Score, SearchContext};

pub const MATE: i32 = 32_000;
pub const INFINITY: i32 = MATE + 1;
/// Deepest ply a search can reach; mate scores stay above `MATE - MAX_PLY`.
pub const MAX_PLY: u32 = 128;

/// Converts an internal score to the UCI form.
pub fn to_uci_score(score: i32) -> Score {
    if score.abs() > MATE - MAX_PLY as i32 {
        let plies = MATE - score.abs();
        let moves = (plies + 1) / 2;
        Score::Mate(if score > 0 { moves } else { -moves })
    } else {
        Score::Cp(score)
    }
}

/// State shared by every node of one iteration.
pub struct Search<'b, 'c, 'a> {
    pub board: &'b mut Board,
    pub context: &'c mut SearchContext<'a>,
    /// Hashes of the game so far followed by the current search path; the
    /// last entry is the current position.
    pub path: Vec<u64>,
}

impl Search<'_, '_, '_> {
    /// Searches `self.board` to `depth` plies and returns its score. `pv` receives
    /// the best line found. `hint` is the principal variation of the previous
    /// iteration while the search is still following it.
    ///
    /// Returns 0 as soon as the search is aborted; the caller must check
    /// `context.aborted()` and discard the result.
    pub fn negamax(
        &mut self,
        depth: u32,
        ply: u32,
        mut alpha: i32,
        beta: i32,
        hint: Option<&[Move]>,
        pv: &mut Vec<Move>,
    ) -> i32 {
        pv.clear();
        if ply > 0 && self.is_draw() {
            return 0;
        }
        if depth == 0 || ply >= MAX_PLY {
            return evaluate(self.board);
        }

        let mut moves = legal_moves(self.board);
        if moves.is_empty() {
            return if in_check(self.board, self.board.side_to_move) {
                -(MATE - ply as i32)
            } else {
                0
            };
        }
        let hint_move = hint.and_then(|line| line.first().copied());
        order_moves(self.board, &mut moves, hint_move);

        let mut best = -INFINITY;
        let mut child_pv = Vec::new();
        for chess_move in moves {
            let child_hint = match hint {
                Some(line) if Some(chess_move) == hint_move => Some(&line[1..]),
                _ => None,
            };
            let undo = self.board.make_move(chess_move);
            self.context.visit(ply + 1);
            self.path.push(position_hash(self.board));
            let score = -self.negamax(depth - 1, ply + 1, -beta, -alpha, child_hint, &mut child_pv);
            self.path.pop();
            self.board.unmake_move(chess_move, undo);

            if self.context.poll_stop() {
                return 0;
            }
            if score > best {
                best = score;
                if score > alpha {
                    alpha = score;
                    pv.clear();
                    pv.push(chess_move);
                    pv.extend_from_slice(&child_pv);
                }
            }
            if alpha >= beta {
                break;
            }
        }
        best
    }

    /// Draws by the fifty-move rule, insufficient material, or repetition. A
    /// single repetition counts: if it was worth repeating once, it is worth
    /// repeating again, so the result is the same as a threefold repetition.
    fn is_draw(&self) -> bool {
        let board = &*self.board;
        if board.halfmove_clock >= 100 || insufficient_material(board) {
            return true;
        }
        let current = self.path[self.path.len() - 1];
        // Positions before the last capture or pawn move cannot repeat.
        self.path
            .iter()
            .rev()
            .skip(1)
            .take(board.halfmove_clock as usize)
            .any(|&hash| hash == current)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn converts_mate_scores_to_moves() {
        assert_eq!(to_uci_score(MATE - 1), Score::Mate(1));
        assert_eq!(to_uci_score(MATE - 3), Score::Mate(2));
        assert_eq!(to_uci_score(-(MATE - 2)), Score::Mate(-1));
        assert_eq!(to_uci_score(-(MATE - 4)), Score::Mate(-2));
        assert_eq!(to_uci_score(125), Score::Cp(125));
        assert_eq!(to_uci_score(-40), Score::Cp(-40));
    }
}
