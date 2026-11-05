#![allow(dead_code)]

use std::fmt;

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

    /// Neither a capture nor a promotion: the material stays the same.
    pub const fn is_quiet(self) -> bool {
        !matches!(
            self.kind,
            MoveKind::Capture
                | MoveKind::EnPassant
                | MoveKind::Promotion
                | MoveKind::PromotionCapture
        )
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

/// What [`crate::board::Board::make_null_move`] changes besides the side to move.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct NullUndo {
    pub en_passant: Option<Square>,
    pub halfmove_clock: u16,
}

/// More than the 218 legal moves of the richest known position.
pub const MAX_MOVES: usize = 256;

/// The moves of one position, kept on the stack. The search generates a list
/// at every node, so a fixed-size array saves a heap allocation per node.
/// It dereferences to a slice, so it is used like a `Vec<Move>` that cannot
/// grow beyond [`MAX_MOVES`].
#[derive(Clone, Copy)]
pub struct MoveList {
    moves: [Move; MAX_MOVES],
    len: usize,
}

impl MoveList {
    pub const fn new() -> Self {
        Self {
            moves: [Move::new(Square::A1, Square::A1, MoveKind::Quiet); MAX_MOVES],
            len: 0,
        }
    }

    pub fn push(&mut self, chess_move: Move) {
        self.moves[self.len] = chess_move;
        self.len += 1;
    }

    pub fn clear(&mut self) {
        self.len = 0;
    }

    /// Keeps the moves for which `keep` returns true, in their order.
    pub fn retain(&mut self, mut keep: impl FnMut(&Move) -> bool) {
        let mut kept = 0;
        for i in 0..self.len {
            if keep(&self.moves[i]) {
                self.moves[kept] = self.moves[i];
                kept += 1;
            }
        }
        self.len = kept;
    }
}

impl Default for MoveList {
    fn default() -> Self {
        Self::new()
    }
}

impl std::ops::Deref for MoveList {
    type Target = [Move];

    fn deref(&self) -> &[Move] {
        &self.moves[..self.len]
    }
}

impl std::ops::DerefMut for MoveList {
    fn deref_mut(&mut self) -> &mut [Move] {
        &mut self.moves[..self.len]
    }
}

impl fmt::Debug for MoveList {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_list().entries(self.iter()).finish()
    }
}

impl IntoIterator for MoveList {
    type Item = Move;
    type IntoIter = std::iter::Take<std::array::IntoIter<Move, MAX_MOVES>>;

    fn into_iter(self) -> Self::IntoIter {
        self.moves.into_iter().take(self.len)
    }
}

impl<'a> IntoIterator for &'a MoveList {
    type Item = &'a Move;
    type IntoIter = std::slice::Iter<'a, Move>;

    fn into_iter(self) -> Self::IntoIter {
        self.iter()
    }
}
