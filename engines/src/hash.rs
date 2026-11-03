#![allow(dead_code)]

use crate::attacks::pawn_attacks;
use crate::board::{Board, Color, PieceKind, Square};

/// One key per piece kind, color and square. The keys of the pieces on the
/// board are XORed together incrementally by `Board::set_piece` and
/// `Board::clear_square` (see [`Board::piece_hash`]).
static PIECE_KEYS: [[[u64; 64]; 2]; 6] = piece_keys();
static CASTLING_KEYS: [u64; 16] = keys(0x3000);
static EN_PASSANT_KEYS: [u64; 8] = keys(0x4000);
const BLACK_TO_MOVE_KEY: u64 = zobrist_key(0x2000);

pub fn piece_key(piece: PieceKind, color: Color, square: Square) -> u64 {
    PIECE_KEYS[piece.index()][color.index()][square.index() as usize]
}

/// The position's hash: the pieces' keys, kept up to date by the board, plus
/// the keys of the side to move, the castling rights and the en passant file.
pub fn position_hash(board: &Board) -> u64 {
    let mut hash = board.piece_hash();
    if board.side_to_move == Color::Black {
        hash ^= BLACK_TO_MOVE_KEY;
    }
    hash ^= CASTLING_KEYS[board.castling_rights.bits() as usize];
    if let Some(square) = board.en_passant {
        if has_en_passant_capture(board, square) {
            hash ^= EN_PASSANT_KEYS[square.file() as usize];
        }
    }
    hash
}

/// Whether a pawn of the side to move can take en passant on `target`: it
/// stands where a pawn of the other color on `target` would attack.
fn has_en_passant_capture(board: &Board, target: Square) -> bool {
    let side = board.side_to_move;
    let pawns = board.piece_bb(PieceKind::Pawn).0 & board.color_bb(side).0;
    pawn_attacks(target, side.opposite()).0 & pawns != 0
}

const fn piece_keys() -> [[[u64; 64]; 2]; 6] {
    let mut table = [[[0; 64]; 2]; 6];
    let mut piece = 0;
    while piece < 6 {
        let mut color = 0;
        while color < 2 {
            let mut square = 0;
            while square < 64 {
                let index = (piece * 128 + color * 64 + square) as u64;
                table[piece][color][square] = zobrist_key(0x1000 + index);
                square += 1;
            }
            color += 1;
        }
        piece += 1;
    }
    table
}

const fn keys<const N: usize>(base: u64) -> [u64; N] {
    let mut table = [0; N];
    let mut i = 0;
    while i < N {
        table[i] = zobrist_key(base + i as u64);
        i += 1;
    }
    table
}

/// SplitMix64 applied to `value`: a fixed pseudo-random key per feature, so
/// no key table has to be stored.
const fn zobrist_key(value: u64) -> u64 {
    let mut key = value.wrapping_add(0x9e3779b97f4a7c15);
    key = (key ^ (key >> 30)).wrapping_mul(0xbf58476d1ce4e5b9);
    key = (key ^ (key >> 27)).wrapping_mul(0x94d049bb133111eb);
    key ^ (key >> 31)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fen::parse_fen;

    use crate::movegen::legal_moves;

    /// The hash computed from scratch, by scanning the board.
    fn full_piece_hash(board: &Board) -> u64 {
        Square::ALL
            .into_iter()
            .filter_map(|square| {
                board
                    .piece_at(square)
                    .map(|(piece, color)| piece_key(piece, color, square))
            })
            .fold(0, |hash, key| hash ^ key)
    }

    fn assert_incremental_hash(board: &mut Board, depth: u32) {
        assert_eq!(board.piece_hash(), full_piece_hash(board));
        if depth == 0 {
            return;
        }
        for chess_move in legal_moves(board) {
            let undo = board.make_move(chess_move);
            assert_incremental_hash(board, depth - 1);
            board.unmake_move(chess_move, undo);
        }
    }

    #[test]
    fn incremental_hash_matches_full_hash() {
        // Castling, en passant, promotions and captures of every kind.
        for fen in [
            "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
            "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
            "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
        ] {
            assert_incremental_hash(&mut parse_fen(fen).unwrap(), 3);
        }
    }

    #[test]
    fn identical_positions_have_identical_hashes() {
        let first = parse_fen("8/8/8/8/8/8/4K3/7k w - - 0 1").unwrap();
        let second = parse_fen("8/8/8/8/8/8/4K3/7k w - - 19 42").unwrap();
        assert_eq!(position_hash(&first), position_hash(&second));
    }

    #[test]
    fn side_to_move_changes_hash() {
        let white = parse_fen("8/8/8/8/8/8/4K3/7k w - - 0 1").unwrap();
        let black = parse_fen("8/8/8/8/8/8/4K3/7k b - - 0 1").unwrap();
        assert_ne!(position_hash(&white), position_hash(&black));
    }

    #[test]
    fn irrelevant_en_passant_does_not_change_hash() {
        let without = parse_fen("8/8/8/8/8/8/4K3/7k w - - 0 1").unwrap();
        let with_target = parse_fen("8/8/8/8/8/8/4K3/7k w - e6 0 1").unwrap();
        assert_eq!(position_hash(&without), position_hash(&with_target));
    }
}
