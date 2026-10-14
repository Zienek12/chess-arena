#![allow(dead_code)]

use std::fmt;

use crate::moves::{Move, MoveKind, Undo};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Bitboard(pub u64);

impl Bitboard {
    pub const fn empty() -> Self {
        Self(0)
    }

    pub const fn from_square(square: Square) -> Self {
        Self(1u64 << square.index())
    }

    pub fn set_bit(&mut self, square: Square) {
        self.0 |= 1u64 << square.index();
    }

    pub fn clear_bit(&mut self, square: Square) {
        self.0 &= !(1u64 << square.index());
    }

    pub fn test_bit(self, square: Square) -> bool {
        (self.0 & (1u64 << square.index())) != 0
    }

    pub fn count_bits(self) -> u32 {
        self.0.count_ones()
    }

    pub fn lsb_index(self) -> Option<u32> {
        if self.0 == 0 {
            None
        } else {
            Some(self.0.trailing_zeros())
        }
    }

    pub fn pop_lsb(&mut self) -> Option<Square> {
        let index = self.lsb_index()?;
        self.0 &= self.0 - 1;
        Some(Square::from_index(index as u8))
    }
}

impl fmt::Display for Bitboard {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for rank in (0u8..8).rev() {
            for file in 0u8..8 {
                let square = Square::from_coords(file, rank);
                let ch = if self.test_bit(square) { '1' } else { '.' };
                write!(f, "{} ", ch)?;
            }
            if rank != 0 {
                writeln!(f)?;
            }
        }
        Ok(())
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Color {
    White = 0,
    Black = 1,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct CastlingRights(u8);

impl CastlingRights {
    pub const NONE: Self = Self(0);
    pub const WHITE_KING_SIDE: Self = Self(1 << 0);
    pub const WHITE_QUEEN_SIDE: Self = Self(1 << 1);
    pub const BLACK_KING_SIDE: Self = Self(1 << 2);
    pub const BLACK_QUEEN_SIDE: Self = Self(1 << 3);

    pub const fn new(value: u8) -> Self {
        Self(value & 0x0f)
    }

    pub const fn contains(self, rights: Self) -> bool {
        self.0 & rights.0 == rights.0
    }

    pub const fn remove(self, rights: Self) -> Self {
        Self(self.0 & !rights.0)
    }

    pub const fn bits(self) -> u8 {
        self.0
    }
}

impl Color {
    pub const ALL: [Color; 2] = [Color::White, Color::Black];

    pub const fn index(self) -> usize {
        self as usize
    }

    pub const fn opposite(self) -> Self {
        match self {
            Color::White => Color::Black,
            Color::Black => Color::White,
        }
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum PieceKind {
    Pawn = 0,
    Knight = 1,
    Bishop = 2,
    Rook = 3,
    Queen = 4,
    King = 5,
}

impl PieceKind {
    pub const ALL: [PieceKind; 6] = [
        PieceKind::Pawn,
        PieceKind::Knight,
        PieceKind::Bishop,
        PieceKind::Rook,
        PieceKind::Queen,
        PieceKind::King,
    ];

    pub const fn index(self) -> usize {
        self as usize
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
#[repr(u8)]
pub enum Square {
    A1 = 0,
    B1 = 1,
    C1 = 2,
    D1 = 3,
    E1 = 4,
    F1 = 5,
    G1 = 6,
    H1 = 7,
    A2 = 8,
    B2 = 9,
    C2 = 10,
    D2 = 11,
    E2 = 12,
    F2 = 13,
    G2 = 14,
    H2 = 15,
    A3 = 16,
    B3 = 17,
    C3 = 18,
    D3 = 19,
    E3 = 20,
    F3 = 21,
    G3 = 22,
    H3 = 23,
    A4 = 24,
    B4 = 25,
    C4 = 26,
    D4 = 27,
    E4 = 28,
    F4 = 29,
    G4 = 30,
    H4 = 31,
    A5 = 32,
    B5 = 33,
    C5 = 34,
    D5 = 35,
    E5 = 36,
    F5 = 37,
    G5 = 38,
    H5 = 39,
    A6 = 40,
    B6 = 41,
    C6 = 42,
    D6 = 43,
    E6 = 44,
    F6 = 45,
    G6 = 46,
    H6 = 47,
    A7 = 48,
    B7 = 49,
    C7 = 50,
    D7 = 51,
    E7 = 52,
    F7 = 53,
    G7 = 54,
    H7 = 55,
    A8 = 56,
    B8 = 57,
    C8 = 58,
    D8 = 59,
    E8 = 60,
    F8 = 61,
    G8 = 62,
    H8 = 63,
}

impl Square {
    pub const ALL: [Square; 64] = [
        Square::A1,
        Square::B1,
        Square::C1,
        Square::D1,
        Square::E1,
        Square::F1,
        Square::G1,
        Square::H1,
        Square::A2,
        Square::B2,
        Square::C2,
        Square::D2,
        Square::E2,
        Square::F2,
        Square::G2,
        Square::H2,
        Square::A3,
        Square::B3,
        Square::C3,
        Square::D3,
        Square::E3,
        Square::F3,
        Square::G3,
        Square::H3,
        Square::A4,
        Square::B4,
        Square::C4,
        Square::D4,
        Square::E4,
        Square::F4,
        Square::G4,
        Square::H4,
        Square::A5,
        Square::B5,
        Square::C5,
        Square::D5,
        Square::E5,
        Square::F5,
        Square::G5,
        Square::H5,
        Square::A6,
        Square::B6,
        Square::C6,
        Square::D6,
        Square::E6,
        Square::F6,
        Square::G6,
        Square::H6,
        Square::A7,
        Square::B7,
        Square::C7,
        Square::D7,
        Square::E7,
        Square::F7,
        Square::G7,
        Square::H7,
        Square::A8,
        Square::B8,
        Square::C8,
        Square::D8,
        Square::E8,
        Square::F8,
        Square::G8,
        Square::H8,
    ];

    pub const fn index(self) -> u8 {
        self as u8
    }

    pub const fn from_index(index: u8) -> Self {
        Self::ALL[index as usize]
    }

    pub const fn from_coords(file: u8, rank: u8) -> Self {
        Self::from_index(rank * 8 + file)
    }

    pub const fn file(self) -> u8 {
        self.index() % 8
    }

    pub const fn rank(self) -> u8 {
        self.index() / 8
    }

    pub fn name(self) -> [char; 2] {
        [
            char::from(b'a' + self.file()),
            char::from(b'1' + self.rank()),
        ]
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Board {
    pub piece_bitboards: [Bitboard; 6],
    pub color_bitboards: [Bitboard; 2],
    pub side_to_move: Color,
    pub castling_rights: CastlingRights,
    pub en_passant: Option<Square>,
    pub halfmove_clock: u16,
    pub fullmove_number: u16,
    squares: [Option<(PieceKind, Color)>; 64],
}

impl Board {
    pub const fn empty() -> Self {
        Self {
            piece_bitboards: [Bitboard::empty(); 6],
            color_bitboards: [Bitboard::empty(); 2],
            side_to_move: Color::White,
            castling_rights: CastlingRights::NONE,
            en_passant: None,
            halfmove_clock: 0,
            fullmove_number: 1,
            squares: [None; 64],
        }
    }

    pub fn make_move(&mut self, chess_move: Move) -> Undo {
        let moving_piece = self
            .piece_at(chess_move.from)
            .expect("move source must contain a piece");
        let undo = Undo {
            captured: self.piece_at(chess_move.to),
            castling_rights: self.castling_rights,
            en_passant: self.en_passant,
            halfmove_clock: self.halfmove_clock,
            fullmove_number: self.fullmove_number,
        };

        self.en_passant = None;
        self.clear_square(chess_move.from);
        self.clear_square(chess_move.to);

        match chess_move.kind {
            MoveKind::Quiet | MoveKind::Capture => {
                self.set_piece(chess_move.to, moving_piece.0, moving_piece.1);
            }
            MoveKind::DoublePawnPush => {
                self.set_piece(chess_move.to, moving_piece.0, moving_piece.1);
                self.en_passant = Some(Square::from_coords(
                    chess_move.from.file(),
                    (chess_move.from.rank() + chess_move.to.rank()) / 2,
                ));
            }
            MoveKind::Promotion | MoveKind::PromotionCapture => {
                self.set_piece(
                    chess_move.to,
                    chess_move.promotion.expect("promotion move needs a piece"),
                    moving_piece.1,
                );
            }
            MoveKind::EnPassant => {
                let captured_rank = chess_move.to.rank() as i8
                    + if moving_piece.1 == Color::White {
                        -1
                    } else {
                        1
                    };
                self.clear_square(Square::from_coords(
                    chess_move.to.file(),
                    captured_rank as u8,
                ));
                self.set_piece(chess_move.to, moving_piece.0, moving_piece.1);
            }
            MoveKind::KingCastle => {
                self.set_piece(chess_move.to, moving_piece.0, moving_piece.1);
                let (rook_from, rook_to) = match moving_piece.1 {
                    Color::White => (Square::H1, Square::F1),
                    Color::Black => (Square::H8, Square::F8),
                };
                self.move_piece(rook_from, rook_to);
            }
            MoveKind::QueenCastle => {
                self.set_piece(chess_move.to, moving_piece.0, moving_piece.1);
                let (rook_from, rook_to) = match moving_piece.1 {
                    Color::White => (Square::A1, Square::D1),
                    Color::Black => (Square::A8, Square::D8),
                };
                self.move_piece(rook_from, rook_to);
            }
        }

        self.update_castling_rights(chess_move.from, chess_move.to, moving_piece, undo.captured);
        if moving_piece.0 == PieceKind::Pawn || undo.captured.is_some() {
            self.halfmove_clock = 0;
        } else {
            self.halfmove_clock += 1;
        }
        if self.side_to_move == Color::Black {
            self.fullmove_number += 1;
        }
        self.side_to_move = self.side_to_move.opposite();
        undo
    }

    pub fn unmake_move(&mut self, chess_move: Move, undo: Undo) {
        self.side_to_move = self.side_to_move.opposite();
        self.castling_rights = undo.castling_rights;
        self.en_passant = undo.en_passant;
        self.halfmove_clock = undo.halfmove_clock;
        self.fullmove_number = undo.fullmove_number;

        let moving_piece = self
            .piece_at(chess_move.to)
            .expect("move destination must contain a piece");
        self.clear_square(chess_move.to);

        match chess_move.kind {
            MoveKind::Quiet | MoveKind::Capture | MoveKind::DoublePawnPush => {
                self.set_piece(chess_move.from, moving_piece.0, moving_piece.1);
            }
            MoveKind::Promotion | MoveKind::PromotionCapture => {
                self.set_piece(chess_move.from, PieceKind::Pawn, moving_piece.1);
            }
            MoveKind::EnPassant => {
                self.set_piece(chess_move.from, PieceKind::Pawn, moving_piece.1);
                let captured_rank = chess_move.to.rank() as i8
                    + if moving_piece.1 == Color::White {
                        -1
                    } else {
                        1
                    };
                self.set_piece(
                    Square::from_coords(chess_move.to.file(), captured_rank as u8),
                    PieceKind::Pawn,
                    moving_piece.1.opposite(),
                );
            }
            MoveKind::KingCastle => {
                self.set_piece(chess_move.from, PieceKind::King, moving_piece.1);
                let (rook_from, rook_to) = match moving_piece.1 {
                    Color::White => (Square::H1, Square::F1),
                    Color::Black => (Square::H8, Square::F8),
                };
                self.move_piece(rook_to, rook_from);
            }
            MoveKind::QueenCastle => {
                self.set_piece(chess_move.from, PieceKind::King, moving_piece.1);
                let (rook_from, rook_to) = match moving_piece.1 {
                    Color::White => (Square::A1, Square::D1),
                    Color::Black => (Square::A8, Square::D8),
                };
                self.move_piece(rook_to, rook_from);
            }
        }

        if let Some((piece, color)) = undo.captured {
            self.set_piece(chess_move.to, piece, color);
        }
    }

    fn move_piece(&mut self, from: Square, to: Square) {
        if let Some((piece, color)) = self.piece_at(from) {
            self.clear_square(from);
            self.clear_square(to);
            self.set_piece(to, piece, color);
        }
    }

    fn update_castling_rights(
        &mut self,
        from: Square,
        to: Square,
        moving_piece: (PieceKind, Color),
        captured: Option<(PieceKind, Color)>,
    ) {
        if moving_piece.0 == PieceKind::King {
            self.castling_rights = self.castling_rights.remove(match moving_piece.1 {
                Color::White => CastlingRights::new(0b0011),
                Color::Black => CastlingRights::new(0b1100),
            });
        }

        let rook_rights = [
            (Square::A1, CastlingRights::WHITE_QUEEN_SIDE),
            (Square::H1, CastlingRights::WHITE_KING_SIDE),
            (Square::A8, CastlingRights::BLACK_QUEEN_SIDE),
            (Square::H8, CastlingRights::BLACK_KING_SIDE),
        ];
        for (square, rights) in rook_rights {
            let moving_rook = moving_piece.0 == PieceKind::Rook && from == square;
            let captured_rook =
                captured == Some((PieceKind::Rook, self.side_to_move.opposite())) && to == square;
            if moving_rook || captured_rook {
                self.castling_rights = self.castling_rights.remove(rights);
            }
        }
    }

    pub fn occupancy(self) -> Bitboard {
        Bitboard(
            self.color_bitboards[Color::White.index()].0
                | self.color_bitboards[Color::Black.index()].0,
        )
    }

    pub fn piece_bb(&self, piece: PieceKind) -> Bitboard {
        self.piece_bitboards[piece.index()]
    }

    pub fn color_bb(&self, color: Color) -> Bitboard {
        self.color_bitboards[color.index()]
    }

    pub fn piece_at(&self, square: Square) -> Option<(PieceKind, Color)> {
        self.squares[square.index() as usize]
    }

    pub fn clear_square(&mut self, square: Square) {
        if let Some((piece, color)) = self.squares[square.index() as usize] {
            self.piece_bitboards[piece.index()].clear_bit(square);
            self.color_bitboards[color.index()].clear_bit(square);
            self.squares[square.index() as usize] = None;
        }
    }

    pub fn set_piece(&mut self, square: Square, piece: PieceKind, color: Color) {
        self.clear_square(square);
        self.piece_bitboards[piece.index()].set_bit(square);
        self.color_bitboards[color.index()].set_bit(square);
        self.squares[square.index() as usize] = Some((piece, color));
    }

    pub fn remove_piece(&mut self, square: Square) {
        self.clear_square(square);
    }

    pub fn piece_count(&self, piece: PieceKind) -> u32 {
        self.piece_bitboards[piece.index()].count_bits()
    }

    pub fn color_count(&self, color: Color) -> u32 {
        self.color_bitboards[color.index()].count_bits()
    }
}

impl fmt::Display for Board {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        for rank in (0u8..8).rev() {
            write!(f, "{} ", rank + 1)?;
            for file in 0u8..8 {
                let square = Square::from_coords(file, rank);
                let ch = match self.piece_at(square) {
                    Some((PieceKind::Pawn, Color::White)) => 'P',
                    Some((PieceKind::Knight, Color::White)) => 'N',
                    Some((PieceKind::Bishop, Color::White)) => 'B',
                    Some((PieceKind::Rook, Color::White)) => 'R',
                    Some((PieceKind::Queen, Color::White)) => 'Q',
                    Some((PieceKind::King, Color::White)) => 'K',
                    Some((PieceKind::Pawn, Color::Black)) => 'p',
                    Some((PieceKind::Knight, Color::Black)) => 'n',
                    Some((PieceKind::Bishop, Color::Black)) => 'b',
                    Some((PieceKind::Rook, Color::Black)) => 'r',
                    Some((PieceKind::Queen, Color::Black)) => 'q',
                    Some((PieceKind::King, Color::Black)) => 'k',
                    None => '.',
                };
                write!(f, "{} ", ch)?;
            }
            if rank != 0 {
                writeln!(f)?;
            }
        }
        writeln!(f)?;
        writeln!(f, "  a b c d e f g h")?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::moves::{Move, MoveKind};

    fn assert_round_trip(mut board: Board, chess_move: Move) {
        let original = board;
        let undo = board.make_move(chess_move);
        board.unmake_move(chess_move, undo);
        assert_eq!(board, original);
    }

    #[test]
    fn quiet_move_round_trip() {
        let mut board = Board::empty();
        board.set_piece(Square::E2, PieceKind::Pawn, Color::White);
        assert_round_trip(board, Move::new(Square::E2, Square::E3, MoveKind::Quiet));
    }

    #[test]
    fn capture_round_trip() {
        let mut board = Board::empty();
        board.set_piece(Square::E2, PieceKind::Knight, Color::White);
        board.set_piece(Square::F4, PieceKind::Pawn, Color::Black);
        assert_round_trip(board, Move::new(Square::E2, Square::F4, MoveKind::Capture));
    }

    #[test]
    fn double_pawn_push_restores_en_passant() {
        let mut board = Board::empty();
        board.set_piece(Square::E2, PieceKind::Pawn, Color::White);
        assert_round_trip(
            board,
            Move::new(Square::E2, Square::E4, MoveKind::DoublePawnPush),
        );
    }

    #[test]
    fn promotion_capture_round_trip() {
        let mut board = Board::empty();
        board.set_piece(Square::E7, PieceKind::Pawn, Color::White);
        board.set_piece(Square::F8, PieceKind::Rook, Color::Black);
        assert_round_trip(
            board,
            Move::promotion(Square::E7, Square::F8, PieceKind::Queen, true),
        );
    }

    #[test]
    fn en_passant_round_trip() {
        let mut board = Board::empty();
        board.set_piece(Square::E5, PieceKind::Pawn, Color::White);
        board.set_piece(Square::D5, PieceKind::Pawn, Color::Black);
        board.en_passant = Some(Square::D6);
        assert_round_trip(
            board,
            Move::new(Square::E5, Square::D6, MoveKind::EnPassant),
        );
    }

    #[test]
    fn castling_round_trip() {
        let mut board = Board::empty();
        board.set_piece(Square::E1, PieceKind::King, Color::White);
        board.set_piece(Square::H1, PieceKind::Rook, Color::White);
        board.castling_rights = CastlingRights::WHITE_KING_SIDE;
        assert_round_trip(
            board,
            Move::new(Square::E1, Square::G1, MoveKind::KingCastle),
        );
    }

    #[test]
    fn queen_side_castling_round_trip() {
        let mut board = Board::empty();
        board.set_piece(Square::E1, PieceKind::King, Color::White);
        board.set_piece(Square::A1, PieceKind::Rook, Color::White);
        board.castling_rights = CastlingRights::WHITE_QUEEN_SIDE;
        assert_round_trip(
            board,
            Move::new(Square::E1, Square::C1, MoveKind::QueenCastle),
        );
    }
}
