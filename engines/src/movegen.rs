#![allow(dead_code)]

use crate::attacks::{
    bishop_attacks, is_square_attacked, is_square_attacked_through, piece_attacks, rook_attacks,
};
use crate::board::{Bitboard, Board, CastlingRights, Color, PieceKind, Square};
use crate::moves::{Move, MoveKind, MoveList};

pub fn pseudo_legal_moves(board: &Board) -> MoveList {
    let side = board.side_to_move;
    let own = board.color_bb(side);
    let enemy = board.color_bb(side.opposite());
    let occupancy = board.occupancy();
    let mut moves = MoveList::new();

    // Pieces are visited from a1 to h8 and targets in the same order, so the
    // moves come out in a fixed order that move ordering then keeps for
    // equally scored moves.
    let mut pieces = own;
    while let Some(square) = pieces.pop_lsb() {
        let Some((piece, _)) = board.piece_at(square) else {
            continue;
        };

        if piece == PieceKind::Pawn {
            generate_pawn_moves(board, square, side, &mut moves);
            continue;
        }

        let attacks = piece_attacks(piece, square, side, occupancy);
        let mut targets = Bitboard(attacks.0 & !own.0);
        while let Some(target) = targets.pop_lsb() {
            let kind = if enemy.test_bit(target) {
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

/// Keeps the pseudo-legal moves that do not leave the own king attacked.
///
/// Playing every move and testing the king is always right but slow, so it is
/// only done for the moves where the answer is not obvious. When the king is
/// not in check, a move of a piece that is not pinned to the king cannot
/// expose it, and a king move is legal when its target is not attacked.
/// Checks, pins, en passant (which removes two pieces from a rank) and
/// castling are rare and still decided by playing the move.
pub fn legal_moves(board: &mut Board) -> MoveList {
    let side = board.side_to_move;
    let enemy = side.opposite();
    let mut moves = pseudo_legal_moves(board);
    let Some(king) = find_king(board, side) else {
        moves.clear();
        return moves;
    };

    let king_in_check = is_square_attacked(board, king, enemy);
    let pinned = pinned_pieces(board, king, side);
    let without_king = Bitboard(board.occupancy().0 & !Bitboard::from_square(king).0);

    moves.retain(|&chess_move| match chess_move.kind {
        MoveKind::KingCastle | MoveKind::QueenCastle => {
            !is_castle_through_check(board, chess_move, enemy)
                && leaves_king_safe(board, chess_move, side)
        }
        _ if chess_move.from == king => {
            !is_square_attacked_through(board, chess_move.to, enemy, without_king)
        }
        MoveKind::EnPassant => leaves_king_safe(board, chess_move, side),
        _ if king_in_check || pinned.test_bit(chess_move.from) => {
            leaves_king_safe(board, chess_move, side)
        }
        _ => true,
    });
    moves
}

fn leaves_king_safe(board: &mut Board, chess_move: Move, side: Color) -> bool {
    let undo = board.make_move(chess_move);
    let safe = !in_check(board, side);
    board.unmake_move(chess_move, undo);
    safe
}

/// Own pieces standing between the king and an enemy bishop, rook or queen
/// on the same line, with nothing else between them. Moving one of them may
/// expose the king.
fn pinned_pieces(board: &Board, king: Square, side: Color) -> Bitboard {
    let own = board.color_bb(side).0;
    let enemy = board.color_bb(side.opposite()).0;
    let occupancy = board.occupancy();
    let queens = board.piece_bb(PieceKind::Queen).0;
    let rook_sliders = (board.piece_bb(PieceKind::Rook).0 | queens) & enemy;
    let bishop_sliders = (board.piece_bb(PieceKind::Bishop).0 | queens) & enemy;

    let mut pinned = 0;
    for (attacks, sliders) in [
        (
            rook_attacks as fn(Square, Bitboard) -> Bitboard,
            rook_sliders,
        ),
        (bishop_attacks, bishop_sliders),
    ] {
        // The own pieces the king sees first on each ray...
        let visible = attacks(king, occupancy).0;
        let blockers = visible & own;
        // ...and the enemy sliders right behind them.
        let behind = attacks(king, Bitboard(occupancy.0 & !blockers)).0 & !visible;
        let mut pinners = Bitboard(behind & sliders);
        while let Some(pinner) = pinners.pop_lsb() {
            // The king and the pinner both see the pinned piece between them.
            pinned |= attacks(pinner, occupancy).0 & blockers;
        }
    }
    Bitboard(pinned)
}

pub fn in_check(board: &Board, color: Color) -> bool {
    find_king(board, color)
        .map(|square| is_square_attacked(board, square, color.opposite()))
        .unwrap_or(true)
}

fn find_king(board: &Board, color: Color) -> Option<Square> {
    let mut kings = Bitboard(board.piece_bb(PieceKind::King).0 & board.color_bb(color).0);
    kings.pop_lsb()
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

fn generate_pawn_moves(board: &Board, from: Square, color: Color, moves: &mut MoveList) {
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
    moves: &mut MoveList,
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

fn generate_castling_moves(board: &Board, color: Color, moves: &mut MoveList) {
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
