#![allow(dead_code)]

use crate::board::{Board, Color, PieceKind};

pub fn position_hash(board: &Board) -> u64 {
    let mut hash = 0u64;
    for square in crate::board::Square::ALL {
        if let Some((piece, color)) = board.piece_at(square) {
            let index =
                piece.index() as u64 * 128 + color.index() as u64 * 64 + square.index() as u64;
            hash ^= zobrist_key(0x1000 + index);
        }
    }

    if board.side_to_move == Color::Black {
        hash ^= zobrist_key(0x2000);
    }
    hash ^= zobrist_key(0x3000 + board.castling_rights.bits() as u64);
    if let Some(square) = board.en_passant {
        if has_en_passant_capture(board, square) {
            hash ^= zobrist_key(0x4000 + square.file() as u64);
        }
    }
    hash
}

fn has_en_passant_capture(board: &Board, target: crate::board::Square) -> bool {
    let pawn_rank = target.rank() as i8
        + if board.side_to_move == Color::White {
            -1
        } else {
            1
        };
    if !(0..8).contains(&pawn_rank) {
        return false;
    }

    for file in [target.file() as i8 - 1, target.file() as i8 + 1] {
        if (0..8).contains(&file)
            && board.piece_at(crate::board::Square::from_coords(
                file as u8,
                pawn_rank as u8,
            )) == Some((PieceKind::Pawn, board.side_to_move))
        {
            return true;
        }
    }
    false
}

fn zobrist_key(value: u64) -> u64 {
    let mut key = value.wrapping_add(0x9e3779b97f4a7c15);
    key = (key ^ (key >> 30)).wrapping_mul(0xbf58476d1ce4e5b9);
    key = (key ^ (key >> 27)).wrapping_mul(0x94d049bb133111eb);
    key ^ (key >> 31)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fen::parse_fen;

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
