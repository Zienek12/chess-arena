//! Move ordering. Alpha-beta prunes most when the best move is searched
//! first, so likely good moves go to the front.

use std::cmp::Reverse;

use crate::board::{Board, PieceKind};
use crate::eval::PIECE_VALUES;
use crate::moves::{Move, MoveKind};

const HINT_SCORE: i32 = 1_000_000;
const TABLE_MOVE_SCORE: i32 = 500_000;
const CAPTURE_SCORE: i32 = 100_000;
const PROMOTION_SCORE: i32 = 90_000;
const MAX_MOVES: usize = 256;

/// Orders `moves` best first: the `hint` (the principal variation move from
/// the previous iteration), the `table_move` (the best move stored in the
/// transposition table), then captures by MVV-LVA (most valuable victim,
/// least valuable attacker), then promotions, then quiet moves.
///
/// The sort is stable: equally scored moves, such as all quiet moves, keep
/// the order of the move generator.
pub fn order_moves(
    board: &Board,
    moves: &mut [Move],
    hint: Option<Move>,
    table_move: Option<Move>,
) {
    // A position has at most 218 legal moves; longer lists are only sorted
    // in tests and take the allocating path.
    if moves.len() > MAX_MOVES {
        moves.sort_by_cached_key(|&chess_move| {
            Reverse(move_score(board, chess_move, hint, table_move))
        });
        return;
    }
    // Insertion sort with the scores kept beside the moves: no allocation,
    // and fast for the few dozen moves of a position.
    let mut scores = [0; MAX_MOVES];
    for i in 0..moves.len() {
        let (chess_move, score) = (moves[i], move_score(board, moves[i], hint, table_move));
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

fn move_score(
    board: &Board,
    chess_move: Move,
    hint: Option<Move>,
    table_move: Option<Move>,
) -> i32 {
    if hint == Some(chess_move) {
        return HINT_SCORE;
    }
    if table_move == Some(chess_move) {
        return TABLE_MOVE_SCORE;
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
        order_moves(&board, &mut moves, hint, None);
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
        order_moves(&board, &mut moves, hint, table_move);
        assert_eq!(moves[0].to_uci(), "e1f2");
        assert_eq!(moves[1].to_uci(), "e1e2");
        assert_eq!(moves[2].to_uci(), "e4d5");
    }

    #[test]
    fn hint_comes_first() {
        let moves = ordered("4k3/8/8/3q4/4P3/8/8/3RK3 w - - 0 1", Some("e1f2"));
        assert_eq!(moves[0], "e1f2");
    }
}
