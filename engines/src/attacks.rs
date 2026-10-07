#![allow(dead_code)]

use crate::board::{Bitboard, Board, Color, PieceKind, Square};

fn add_square(attacks: &mut Bitboard, file: i8, rank: i8) {
    if (0..8).contains(&file) && (0..8).contains(&rank) {
        attacks.set_bit(Square::from_coords(file as u8, rank as u8));
    }
}

pub fn pawn_attacks(square: Square, color: Color) -> Bitboard {
    let mut attacks = Bitboard::empty();
    let file = square.file() as i8;
    let rank = square.rank() as i8;
    let rank_delta = match color {
        Color::White => 1,
        Color::Black => -1,
    };
    add_square(&mut attacks, file - 1, rank + rank_delta);
    add_square(&mut attacks, file + 1, rank + rank_delta);
    attacks
}

pub fn knight_attacks(square: Square) -> Bitboard {
    let mut attacks = Bitboard::empty();
    let file = square.file() as i8;
    let rank = square.rank() as i8;
    for (file_delta, rank_delta) in [
        (-2, -1),
        (-2, 1),
        (-1, -2),
        (-1, 2),
        (1, -2),
        (1, 2),
        (2, -1),
        (2, 1),
    ] {
        add_square(&mut attacks, file + file_delta, rank + rank_delta);
    }
    attacks
}

pub fn king_attacks(square: Square) -> Bitboard {
    let mut attacks = Bitboard::empty();
    let file = square.file() as i8;
    let rank = square.rank() as i8;
    for file_delta in -1..=1 {
        for rank_delta in -1..=1 {
            if file_delta != 0 || rank_delta != 0 {
                add_square(&mut attacks, file + file_delta, rank + rank_delta);
            }
        }
    }
    attacks
}

pub fn bishop_attacks(square: Square, occupancy: Bitboard) -> Bitboard {
    sliding_attacks(square, occupancy, &[(-1, -1), (-1, 1), (1, -1), (1, 1)])
}

pub fn rook_attacks(square: Square, occupancy: Bitboard) -> Bitboard {
    sliding_attacks(square, occupancy, &[(-1, 0), (1, 0), (0, -1), (0, 1)])
}

pub fn queen_attacks(square: Square, occupancy: Bitboard) -> Bitboard {
    let mut attacks = bishop_attacks(square, occupancy);
    attacks.0 |= rook_attacks(square, occupancy).0;
    attacks
}

pub fn piece_attacks(
    piece: PieceKind,
    square: Square,
    color: Color,
    occupancy: Bitboard,
) -> Bitboard {
    match piece {
        PieceKind::Pawn => pawn_attacks(square, color),
        PieceKind::Knight => knight_attacks(square),
        PieceKind::Bishop => bishop_attacks(square, occupancy),
        PieceKind::Rook => rook_attacks(square, occupancy),
        PieceKind::Queen => queen_attacks(square, occupancy),
        PieceKind::King => king_attacks(square),
    }
}

pub fn is_square_attacked(board: &Board, target: Square, by_color: Color) -> bool {
    for square in Square::ALL {
        if let Some((piece, color)) = board.piece_at(square) {
            if color == by_color
                && piece_attacks(piece, square, color, board.occupancy()).test_bit(target)
            {
                return true;
            }
        }
    }
    false
}

fn sliding_attacks(square: Square, occupancy: Bitboard, directions: &[(i8, i8)]) -> Bitboard {
    let mut attacks = Bitboard::empty();
    let origin_file = square.file() as i8;
    let origin_rank = square.rank() as i8;

    for &(file_delta, rank_delta) in directions {
        let mut file = origin_file + file_delta;
        let mut rank = origin_rank + rank_delta;
        while (0..8).contains(&file) && (0..8).contains(&rank) {
            let target = Square::from_coords(file as u8, rank as u8);
            attacks.set_bit(target);
            if occupancy.test_bit(target) {
                break;
            }
            file += file_delta;
            rank += rank_delta;
        }
    }
    attacks
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn knight_has_eight_attacks_in_center() {
        assert_eq!(knight_attacks(Square::D4).count_bits(), 8);
    }

    #[test]
    fn king_has_three_attacks_in_corner() {
        assert_eq!(king_attacks(Square::A1).count_bits(), 3);
    }

    #[test]
    fn pawn_attacks_depend_on_color() {
        assert!(pawn_attacks(Square::E4, Color::White).test_bit(Square::D5));
        assert!(pawn_attacks(Square::E4, Color::White).test_bit(Square::F5));
        assert!(pawn_attacks(Square::E4, Color::Black).test_bit(Square::D3));
        assert!(pawn_attacks(Square::E4, Color::Black).test_bit(Square::F3));
    }

    #[test]
    fn rook_attacks_stop_at_blocker() {
        let mut occupancy = Bitboard::empty();
        occupancy.set_bit(Square::D6);
        occupancy.set_bit(Square::F4);
        let attacks = rook_attacks(Square::D4, occupancy);

        assert!(attacks.test_bit(Square::D6));
        assert!(!attacks.test_bit(Square::D7));
        assert!(attacks.test_bit(Square::F4));
        assert!(!attacks.test_bit(Square::G4));
    }

    #[test]
    fn bishop_attacks_are_blocked_on_each_ray() {
        let mut occupancy = Bitboard::empty();
        occupancy.set_bit(Square::F6);
        let attacks = bishop_attacks(Square::D4, occupancy);

        assert!(attacks.test_bit(Square::E5));
        assert!(attacks.test_bit(Square::F6));
        assert!(!attacks.test_bit(Square::G7));
        assert!(attacks.test_bit(Square::C5));
    }

    #[test]
    fn attacked_square_is_detected_from_board() {
        let mut board = Board::empty();
        board.set_piece(Square::E4, PieceKind::Rook, Color::White);
        assert!(is_square_attacked(&board, Square::E8, Color::White));
        assert!(!is_square_attacked(&board, Square::D8, Color::White));
    }
}
