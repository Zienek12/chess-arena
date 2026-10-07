#![allow(dead_code)]

use crate::attacks::{is_square_attacked, piece_attacks};
use crate::board::{Board, CastlingRights, Color, PieceKind, Square};
use crate::moves::{Move, MoveKind};

pub fn pseudo_legal_moves(board: &Board) -> Vec<Move> {
    let side = board.side_to_move;
    let own = board.color_bb(side);
    let occupancy = board.occupancy();
    let mut moves = Vec::new();

    for square in Square::ALL {
        let Some((piece, color)) = board.piece_at(square) else {
            continue;
        };
        if color != side {
            continue;
        }

        if piece == PieceKind::Pawn {
            generate_pawn_moves(board, square, side, &mut moves);
            continue;
        }

        let attacks = piece_attacks(piece, square, side, occupancy);
        let mut targets = Bitboard(attacks.0 & !own.0);
        while let Some(target) = targets.pop_lsb() {
            let kind = if board.piece_at(target).is_some() {
                MoveKind::Capture
            } else {
                MoveKind::Quiet
            };
            moves.push(Move::new(square, target, kind));
        }

        if piece == PieceKind::King {
            generate_castling_moves(board, side, &mut moves);
        }
    }

    moves
}

pub fn legal_moves(board: &mut Board) -> Vec<Move> {
    let side = board.side_to_move;
    let enemy = side.opposite();
    let mut legal = Vec::new();

    for chess_move in pseudo_legal_moves(board) {
        if is_castle_through_check(board, chess_move, enemy) {
            continue;
        }
        let undo = board.make_move(chess_move);
        let king_square = find_king(board, side);
        let remains_in_check = king_square
            .map(|square| is_square_attacked(board, square, enemy))
            .unwrap_or(true);
        board.unmake_move(chess_move, undo);
        if !remains_in_check {
            legal.push(chess_move);
        }
    }

    legal
}

pub fn in_check(board: &Board, color: Color) -> bool {
    find_king(board, color)
        .map(|square| is_square_attacked(board, square, color.opposite()))
        .unwrap_or(true)
}

fn find_king(board: &Board, color: Color) -> Option<Square> {
    Square::ALL
        .into_iter()
        .find(|square| board.piece_at(*square) == Some((PieceKind::King, color)))
}

fn is_castle_through_check(board: &Board, chess_move: Move, enemy: Color) -> bool {
    let path = match chess_move.kind {
        MoveKind::KingCastle => match board.side_to_move {
            Color::White => [Square::E1, Square::F1, Square::G1],
            Color::Black => [Square::E8, Square::F8, Square::G8],
        },
        MoveKind::QueenCastle => match board.side_to_move {
            Color::White => [Square::E1, Square::D1, Square::C1],
            Color::Black => [Square::E8, Square::D8, Square::C8],
        },
        _ => return false,
    };
    path.into_iter()
        .any(|square| is_square_attacked(board, square, enemy))
}

use crate::board::Bitboard;

fn generate_pawn_moves(board: &Board, from: Square, color: Color, moves: &mut Vec<Move>) {
    let direction: i8 = if color == Color::White { 1 } else { -1 };
    let start_rank = if color == Color::White { 1 } else { 6 };
    let promotion_rank = if color == Color::White { 7 } else { 0 };
    let file = from.file() as i8;
    let rank = from.rank() as i8;
    let one_rank = rank + direction;

    if (0..8).contains(&one_rank) {
        let one_step = Square::from_coords(file as u8, one_rank as u8);
        if board.piece_at(one_step).is_none() {
            add_pawn_move(moves, from, one_step, promotion_rank, false);

            if rank == start_rank {
                let two_rank = rank + 2 * direction;
                let two_step = Square::from_coords(file as u8, two_rank as u8);
                if board.piece_at(two_step).is_none() {
                    moves.push(Move::new(from, two_step, MoveKind::DoublePawnPush));
                }
            }
        }
    }

    for target_file in [file - 1, file + 1] {
        if !(0..8).contains(&target_file) || !(0..8).contains(&one_rank) {
            continue;
        }
        let target = Square::from_coords(target_file as u8, one_rank as u8);
        let is_capture = board
            .piece_at(target)
            .map(|(_, target_color)| target_color != color)
            .unwrap_or(false);
        let is_en_passant = board.en_passant == Some(target);
        if is_capture || is_en_passant {
            if is_en_passant {
                moves.push(Move::new(from, target, MoveKind::EnPassant));
            } else {
                add_pawn_move(moves, from, target, promotion_rank, true);
            }
        }
    }
}

fn add_pawn_move(
    moves: &mut Vec<Move>,
    from: Square,
    to: Square,
    promotion_rank: i8,
    capture: bool,
) {
    if to.rank() as i8 == promotion_rank {
        for promotion in [
            PieceKind::Queen,
            PieceKind::Rook,
            PieceKind::Bishop,
            PieceKind::Knight,
        ] {
            moves.push(Move::promotion(from, to, promotion, capture));
        }
    } else {
        moves.push(Move::new(
            from,
            to,
            if capture {
                MoveKind::Capture
            } else {
                MoveKind::Quiet
            },
        ));
    }
}

fn generate_castling_moves(board: &Board, color: Color, moves: &mut Vec<Move>) {
    let (king_square, king_side, queen_side) = match color {
        Color::White => (
            Square::E1,
            CastlingRights::WHITE_KING_SIDE,
            CastlingRights::WHITE_QUEEN_SIDE,
        ),
        Color::Black => (
            Square::E8,
            CastlingRights::BLACK_KING_SIDE,
            CastlingRights::BLACK_QUEEN_SIDE,
        ),
    };
    if board.piece_at(king_square) != Some((PieceKind::King, color)) {
        return;
    }

    if board.castling_rights.contains(king_side) {
        let (rook_square, empty_squares, target) = match color {
            Color::White => (Square::H1, [Square::F1, Square::G1], Square::G1),
            Color::Black => (Square::H8, [Square::F8, Square::G8], Square::G8),
        };
        if board.piece_at(rook_square) == Some((PieceKind::Rook, color))
            && empty_squares
                .iter()
                .all(|square| board.piece_at(*square).is_none())
        {
            moves.push(Move::new(king_square, target, MoveKind::KingCastle));
        }
    }

    if board.castling_rights.contains(queen_side) {
        let (rook_square, empty_squares, target) = match color {
            Color::White => (Square::A1, [Square::B1, Square::C1, Square::D1], Square::C1),
            Color::Black => (Square::A8, [Square::B8, Square::C8, Square::D8], Square::C8),
        };
        if board.piece_at(rook_square) == Some((PieceKind::Rook, color))
            && empty_squares
                .iter()
                .all(|square| board.piece_at(*square).is_none())
        {
            moves.push(Move::new(king_square, target, MoveKind::QueenCastle));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn knight_generates_all_center_moves() {
        let mut board = Board::empty();
        board.set_piece(Square::D4, PieceKind::Knight, Color::White);
        assert_eq!(pseudo_legal_moves(&board).len(), 8);
    }

    #[test]
    fn pawn_generates_single_double_and_capture() {
        let mut board = Board::empty();
        board.set_piece(Square::E2, PieceKind::Pawn, Color::White);
        board.set_piece(Square::D3, PieceKind::Knight, Color::Black);
        let moves = pseudo_legal_moves(&board);

        assert!(moves.contains(&Move::new(Square::E2, Square::E3, MoveKind::Quiet)));
        assert!(moves.contains(&Move::new(Square::E2, Square::E4, MoveKind::DoublePawnPush)));
        assert!(moves.contains(&Move::new(Square::E2, Square::D3, MoveKind::Capture)));
    }

    #[test]
    fn pawn_generates_four_promotions() {
        let mut board = Board::empty();
        board.set_piece(Square::E7, PieceKind::Pawn, Color::White);
        assert_eq!(pseudo_legal_moves(&board).len(), 4);
    }

    #[test]
    fn castling_requires_empty_squares_and_rook() {
        let mut board = Board::empty();
        board.set_piece(Square::E1, PieceKind::King, Color::White);
        board.set_piece(Square::H1, PieceKind::Rook, Color::White);
        board.castling_rights = CastlingRights::WHITE_KING_SIDE;
        let moves = pseudo_legal_moves(&board);

        assert!(moves.contains(&Move::new(Square::E1, Square::G1, MoveKind::KingCastle)));
    }

    #[test]
    fn legal_moves_reject_a_pinned_piece_move() {
        let mut board = Board::empty();
        board.set_piece(Square::E1, PieceKind::King, Color::White);
        board.set_piece(Square::E2, PieceKind::Rook, Color::White);
        board.set_piece(Square::E8, PieceKind::Rook, Color::Black);
        let legal = legal_moves(&mut board);

        assert!(!legal.contains(&Move::new(Square::E2, Square::D2, MoveKind::Quiet)));
    }

    #[test]
    fn legal_moves_reject_castling_through_attack() {
        let mut board = Board::empty();
        board.set_piece(Square::E1, PieceKind::King, Color::White);
        board.set_piece(Square::H1, PieceKind::Rook, Color::White);
        board.set_piece(Square::F8, PieceKind::Rook, Color::Black);
        board.castling_rights = CastlingRights::WHITE_KING_SIDE;
        assert!(!legal_moves(&mut board).contains(&Move::new(
            Square::E1,
            Square::G1,
            MoveKind::KingCastle,
        )));
    }
}
