#![allow(dead_code)]

use crate::board::Board;
use crate::movegen::legal_moves;
use crate::moves::Move;

pub fn perft(board: &mut Board, depth: u8) -> u64 {
    if depth == 0 {
        return 1;
    }

    let moves = legal_moves(board);
    if depth == 1 {
        return moves.len() as u64;
    }

    let mut nodes = 0;
    for chess_move in moves {
        let undo = board.make_move(chess_move);
        nodes += perft(board, depth - 1);
        board.unmake_move(chess_move, undo);
    }
    nodes
}

pub fn divide(board: &mut Board, depth: u8) -> Vec<(Move, u64)> {
    if depth == 0 {
        return Vec::new();
    }

    let mut result = Vec::new();
    for chess_move in legal_moves(board) {
        let undo = board.make_move(chess_move);
        let nodes = perft(board, depth - 1);
        board.unmake_move(chess_move, undo);
        result.push((chess_move, nodes));
    }
    result
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fen::parse_fen;

    const START: &str = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1";

    #[test]
    fn start_position_perft_depth_one() {
        let mut board = parse_fen(START).unwrap();
        assert_eq!(perft(&mut board, 1), 20);
    }

    #[test]
    fn start_position_perft_depth_two() {
        let mut board = parse_fen(START).unwrap();
        assert_eq!(perft(&mut board, 2), 400);
    }

    #[test]
    fn start_position_perft_depth_three() {
        let mut board = parse_fen(START).unwrap();
        assert_eq!(perft(&mut board, 3), 8902);
    }
}
