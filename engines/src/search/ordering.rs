//! Move ordering. Alpha-beta prunes most when the best move is searched
//! first, so likely good moves go to the front.

use std::cmp::Reverse;

use crate::board::{Board, PieceKind};
use crate::eval::PIECE_VALUES;
use crate::moves::{Move, MoveKind};

use super::heuristics::Heuristics;

const HINT_SCORE: i32 = 1_000_000;
const TABLE_MOVE_SCORE: i32 = 500_000;
const CAPTURE_SCORE: i32 = 100_000;
const PROMOTION_SCORE: i32 = 90_000;
const MAX_MOVES: usize = 256;

/// What the search knows about the moves of a position, beyond the moves
/// themselves.
#[derive(Clone, Copy, Default)]
pub struct Priorities<'h> {
    /// The principal variation move from the previous iteration.
    pub hint: Option<Move>,
    /// The best move stored in the transposition table.
    pub table_move: Option<Move>,
    /// What was learned about quiet moves, and the ply of this position.
    pub quiet: Option<(&'h Heuristics, u32)>,
}

/// Orders `moves` best first: the hint, the table move, then captures by
/// MVV-LVA (most valuable victim, least valuable attacker), then promotions,
/// then killer moves, then the other quiet moves by their history score.
///
/// The sort is stable: equally scored moves, such as all quiet moves without
/// a history, keep the order of the move generator.
pub fn order_moves(board: &Board, moves: &mut [Move], priorities: Priorities) {
    // A position has at most 218 legal moves; longer lists are only sorted
    // in tests and take the allocating path.
    if moves.len() > MAX_MOVES {
        moves.sort_by_cached_key(|&chess_move| Reverse(move_score(board, chess_move, priorities)));
        return;
    }
    // Insertion sort with the scores kept beside the moves: no allocation,
    // and fast for the few dozen moves of a position.
    let mut scores = [0; MAX_MOVES];
    for i in 0..moves.len() {
        let (chess_move, score) = (moves[i], move_score(board, moves[i], priorities));
        let mut j = i;
        while j > 0 && scores[j - 1] < score {
            moves[j] = moves[j - 1];
            scores[j] = scores[j - 1];
            j -= 1;
        }
        moves[j] = chess_move;
        scores[j] = score;
    }
}

fn move_score(board: &Board, chess_move: Move, priorities: Priorities) -> i32 {
    if priorities.hint == Some(chess_move) {
        return HINT_SCORE;
    }
    if priorities.table_move == Some(chess_move) {
        return TABLE_MOVE_SCORE;
    }
    if chess_move.is_quiet() {
        return priorities.quiet.map_or(0, |(heuristics, ply)| {
            heuristics.quiet_score(board.side_to_move, chess_move, ply)
        });
    }
    let mut score = 0;
    if let Some(victim) = captured_piece(board, chess_move) {
        let attacker = board
            .piece_at(chess_move.from)
            .map_or(PieceKind::Pawn, |(piece, _)| piece);
        score += CAPTURE_SCORE + PIECE_VALUES[victim.index()] * 10 - PIECE_VALUES[attacker.index()];
    }
    if let Some(promotion) = chess_move.promotion {
        score += PROMOTION_SCORE + PIECE_VALUES[promotion.index()];
    }
    score
}

pub fn captured_piece(board: &Board, chess_move: Move) -> Option<PieceKind> {
    match chess_move.kind {
        MoveKind::EnPassant => Some(PieceKind::Pawn),
        MoveKind::Capture | MoveKind::PromotionCapture => {
            board.piece_at(chess_move.to).map(|(piece, _)| piece)
        }
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fen::parse_fen;
    use crate::movegen::legal_moves;

    fn ordered(fen: &str, hint: Option<&str>) -> Vec<String> {
        let mut board = parse_fen(fen).unwrap();
        let mut moves = legal_moves(&mut board);
        let hint = hint.and_then(|uci| moves.iter().copied().find(|m| m.to_uci() == uci));
        let priorities = Priorities {
            hint,
            ..Priorities::default()
        };
        order_moves(&board, &mut moves, priorities);
        moves
            .into_iter()
            .map(|chess_move| chess_move.to_uci())
            .collect()
    }

    #[test]
    fn captures_most_valuable_victim_first() {
        // The pawn on e4 can take the queen on d5 or the knight on f5.
        let moves = ordered("4k3/8/8/3q1n2/4P3/8/8/4K3 w - - 0 1", None);
        assert_eq!(&moves[..2], ["e4d5", "e4f5"]);
    }

    #[test]
    fn least_valuable_attacker_first() {
        // Both the pawn and the rook can take the queen on d5.
        let moves = ordered("4k3/8/8/3q4/4P3/8/8/3RK3 w - - 0 1", None);
        assert_eq!(&moves[..2], ["e4d5", "d1d5"]);
    }

    #[test]
    fn table_move_comes_after_the_hint_and_before_captures() {
        let mut board = parse_fen("4k3/8/8/3q4/4P3/8/8/3RK3 w - - 0 1").unwrap();
        let mut moves = legal_moves(&mut board);
        let find = |uci: &str| moves.iter().copied().find(|m| m.to_uci() == uci);
        let (hint, table_move) = (find("e1f2"), find("e1e2"));
        let priorities = Priorities {
            hint,
            table_move,
            ..Priorities::default()
        };
        order_moves(&board, &mut moves, priorities);
        assert_eq!(moves[0].to_uci(), "e1f2");
        assert_eq!(moves[1].to_uci(), "e1e2");
        assert_eq!(moves[2].to_uci(), "e4d5");
    }

    #[test]
    fn learned_quiet_moves_come_after_captures_and_promotions() {
        // Ke2 is the killer, Kf2 has a history; the pawn on e4 can capture and
        // the pawn on a7 can promote.
        let mut board = parse_fen("4k3/P7/8/3n4/4P3/8/8/4K3 w - - 0 1").unwrap();
        let mut moves = legal_moves(&mut board);
        let find = |uci: &str| moves.iter().copied().find(|m| m.to_uci() == uci);
        let (killer, liked, disliked) = (
            find("e1e2").unwrap(),
            find("e1f2").unwrap(),
            find("e1d2").unwrap(),
        );
        let mut heuristics = Heuristics::new();
        heuristics.record_cutoff(board.side_to_move, 5, 3, liked, &[disliked]);
        heuristics.record_cutoff(board.side_to_move, 2, 1, killer, &[]);
        let priorities = Priorities {
            quiet: Some((&heuristics, 2)),
            ..Priorities::default()
        };
        order_moves(&board, &mut moves, priorities);

        let names: Vec<String> = moves.iter().map(|m| m.to_uci()).collect();
        assert_eq!(names[0], "e4d5");
        assert_eq!(&names[1..5], ["a7a8q", "a7a8r", "a7a8b", "a7a8n"]);
        // At ply 2 the move with a history at ply 5 is not a killer, but keeps
        // its score from the shared history.
        assert_eq!(&names[5..7], ["e1e2", "e1f2"]);
        assert_eq!(names.last().unwrap(), "e1d2");
    }

    #[test]
    fn hint_comes_first() {
        let moves = ordered("4k3/8/8/3q4/4P3/8/8/3RK3 w - - 0 1", Some("e1f2"));
        assert_eq!(moves[0], "e1f2");
    }
}
