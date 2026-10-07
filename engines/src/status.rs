#![allow(dead_code)]

use crate::board::{Board, PieceKind};
use crate::movegen::{in_check, legal_moves};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum GameStatus {
    Ongoing,
    Checkmate,
    Stalemate,
    FiftyMoveDraw,
    InsufficientMaterial,
}

pub fn game_status(board: &mut Board) -> GameStatus {
    let side = board.side_to_move;
    let moves = legal_moves(board);

    if moves.is_empty() {
        return if in_check(board, side) {
            GameStatus::Checkmate
        } else {
            GameStatus::Stalemate
        };
    }
    if board.halfmove_clock >= 100 {
        return GameStatus::FiftyMoveDraw;
    }
    if insufficient_material(board) {
        return GameStatus::InsufficientMaterial;
    }
    GameStatus::Ongoing
}

fn insufficient_material(board: &Board) -> bool {
    if board.piece_bb(PieceKind::Pawn).0 != 0
        || board.piece_bb(PieceKind::Rook).0 != 0
        || board.piece_bb(PieceKind::Queen).0 != 0
    {
        return false;
    }

    let minor_count = board.piece_count(PieceKind::Bishop) + board.piece_count(PieceKind::Knight);
    minor_count <= 1
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fen::parse_fen;

    #[test]
    fn detects_checkmate() {
        let mut board = parse_fen("7k/6Q1/5K2/8/8/8/8/8 b - - 0 1").unwrap();
        assert_eq!(game_status(&mut board), GameStatus::Checkmate);
    }

    #[test]
    fn detects_stalemate() {
        let mut board = parse_fen("7k/5Q2/6K1/8/8/8/8/8 b - - 0 1").unwrap();
        assert_eq!(game_status(&mut board), GameStatus::Stalemate);
    }

    #[test]
    fn detects_fifty_move_draw() {
        let mut board = parse_fen("7k/8/8/8/8/8/8/K7 w - - 100 1").unwrap();
        assert_eq!(game_status(&mut board), GameStatus::FiftyMoveDraw);
    }

    #[test]
    fn detects_insufficient_material() {
        let mut board = parse_fen("7k/8/8/8/8/8/6B1/K7 w - - 0 1").unwrap();
        assert_eq!(game_status(&mut board), GameStatus::InsufficientMaterial);
    }

    #[test]
    fn detects_ongoing_position() {
        let mut board =
            parse_fen("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1").unwrap();
        assert_eq!(game_status(&mut board), GameStatus::Ongoing);
    }
}
