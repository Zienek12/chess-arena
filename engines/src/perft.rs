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

    // Reference positions and node counts: https://www.chessprogramming.org/Perft_Results
    const START: &str = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1";
    const KIWIPETE: &str = "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1";
    const POSITION_3: &str = "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1";
    const POSITION_4: &str = "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1";
    const POSITION_5: &str = "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8";
    const POSITION_6: &str =
        "r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10";

    fn assert_perft(fen: &str, expected: &[u64]) {
        let mut board = parse_fen(fen).unwrap();
        for (depth, &nodes) in expected.iter().enumerate() {
            let depth = depth as u8 + 1;
            assert_eq!(perft(&mut board, depth), nodes, "{fen} at depth {depth}");
        }
    }

    #[test]
    fn start_position_perft() {
        assert_perft(START, &[20, 400, 8_902, 197_281]);
    }

    #[test]
    fn kiwipete_perft() {
        assert_perft(KIWIPETE, &[48, 2_039, 97_862]);
    }

    #[test]
    fn position_3_perft() {
        assert_perft(POSITION_3, &[14, 191, 2_812, 43_238]);
    }

    #[test]
    fn position_4_perft() {
        assert_perft(POSITION_4, &[6, 264, 9_467]);
    }

    #[test]
    fn position_5_perft() {
        assert_perft(POSITION_5, &[44, 1_486, 62_379]);
    }

    #[test]
    fn position_6_perft() {
        assert_perft(POSITION_6, &[46, 2_079, 89_890]);
    }

    fn assert_deep_perft(fen: &str, depth: u8, nodes: u64) {
        let mut board = parse_fen(fen).unwrap();
        assert_eq!(perft(&mut board, depth), nodes, "{fen} at depth {depth}");
    }

    #[test]
    #[ignore = "deep perft; run with cargo test --release -- --ignored"]
    fn start_position_deep_perft() {
        assert_deep_perft(START, 5, 4_865_609);
    }

    #[test]
    #[ignore = "deep perft; run with cargo test --release -- --ignored"]
    fn kiwipete_deep_perft() {
        assert_deep_perft(KIWIPETE, 5, 193_690_690);
    }

    #[test]
    #[ignore = "deep perft; run with cargo test --release -- --ignored"]
    fn position_3_deep_perft() {
        assert_deep_perft(POSITION_3, 6, 11_030_083);
    }

    #[test]
    #[ignore = "deep perft; run with cargo test --release -- --ignored"]
    fn position_4_deep_perft() {
        assert_deep_perft(POSITION_4, 5, 15_833_292);
    }

    #[test]
    #[ignore = "deep perft; run with cargo test --release -- --ignored"]
    fn position_5_deep_perft() {
        assert_deep_perft(POSITION_5, 4, 2_103_487);
    }

    #[test]
    #[ignore = "deep perft; run with cargo test --release -- --ignored"]
    fn position_6_deep_perft() {
        assert_deep_perft(POSITION_6, 4, 3_894_594);
    }
}
