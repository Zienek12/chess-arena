#![allow(dead_code)]

use crate::board::{CastlingRights, Color, PieceKind, Square};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum MoveKind {
    Quiet,
    Capture,
    DoublePawnPush,
    KingCastle,
    QueenCastle,
    EnPassant,
    Promotion,
    PromotionCapture,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Move {
    pub from: Square,
    pub to: Square,
    pub promotion: Option<PieceKind>,
    pub kind: MoveKind,
}

impl Move {
    pub const fn new(from: Square, to: Square, kind: MoveKind) -> Self {
        Self {
            from,
            to,
            promotion: None,
            kind,
        }
    }

    pub const fn promotion(from: Square, to: Square, piece: PieceKind, capture: bool) -> Self {
        Self {
            from,
            to,
            promotion: Some(piece),
            kind: if capture {
                MoveKind::PromotionCapture
            } else {
                MoveKind::Promotion
            },
        }
    }

    pub fn to_uci(self) -> String {
        let mut value = String::with_capacity(5);
        value.push(char::from(b'a' + self.from.file()));
        value.push(char::from(b'1' + self.from.rank()));
        value.push(char::from(b'a' + self.to.file()));
        value.push(char::from(b'1' + self.to.rank()));
        if let Some(piece) = self.promotion {
            value.push(match piece {
                PieceKind::Knight => 'n',
                PieceKind::Bishop => 'b',
                PieceKind::Rook => 'r',
                PieceKind::Queen => 'q',
                _ => 'q',
            });
        }
        value
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Undo {
    pub captured: Option<(PieceKind, Color)>,
    pub castling_rights: CastlingRights,
    pub en_passant: Option<Square>,
    pub halfmove_clock: u16,
    pub fullmove_number: u16,
}
