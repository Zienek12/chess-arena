//! Static evaluation: material plus piece-square tables (PST).
//!
//! Values and tables follow Tomasz Michniewski's "Simplified Evaluation
//! Function" (https://www.chessprogramming.org/Simplified_Evaluation_Function).
//! The king uses a middlegame and an endgame table, blended by the amount of
//! non-pawn material left, so it shelters early and centralizes late.

use crate::board::{Board, Color, PieceKind, Square};

/// Material values in centipawns, indexed by `PieceKind::index()`.
pub const PIECE_VALUES: [i32; 6] = [100, 320, 330, 500, 900, 0];

// Tables are written as seen from White, rank 8 first, so they read like a
// board diagram. White squares are mirrored vertically to index them.
#[rustfmt::skip]
const PAWN_TABLE: [i32; 64] = [
     0,  0,  0,  0,  0,  0,  0,  0,
    50, 50, 50, 50, 50, 50, 50, 50,
    10, 10, 20, 30, 30, 20, 10, 10,
     5,  5, 10, 25, 25, 10,  5,  5,
     0,  0,  0, 20, 20,  0,  0,  0,
     5, -5,-10,  0,  0,-10, -5,  5,
     5, 10, 10,-20,-20, 10, 10,  5,
     0,  0,  0,  0,  0,  0,  0,  0,
];

#[rustfmt::skip]
const KNIGHT_TABLE: [i32; 64] = [
    -50,-40,-30,-30,-30,-30,-40,-50,
    -40,-20,  0,  0,  0,  0,-20,-40,
    -30,  0, 10, 15, 15, 10,  0,-30,
    -30,  5, 15, 20, 20, 15,  5,-30,
    -30,  0, 15, 20, 20, 15,  0,-30,
    -30,  5, 10, 15, 15, 10,  5,-30,
    -40,-20,  0,  5,  5,  0,-20,-40,
    -50,-40,-30,-30,-30,-30,-40,-50,
];

#[rustfmt::skip]
const BISHOP_TABLE: [i32; 64] = [
    -20,-10,-10,-10,-10,-10,-10,-20,
    -10,  0,  0,  0,  0,  0,  0,-10,
    -10,  0,  5, 10, 10,  5,  0,-10,
    -10,  5,  5, 10, 10,  5,  5,-10,
    -10,  0, 10, 10, 10, 10,  0,-10,
    -10, 10, 10, 10, 10, 10, 10,-10,
    -10,  5,  0,  0,  0,  0,  5,-10,
    -20,-10,-10,-10,-10,-10,-10,-20,
];

#[rustfmt::skip]
const ROOK_TABLE: [i32; 64] = [
     0,  0,  0,  0,  0,  0,  0,  0,
     5, 10, 10, 10, 10, 10, 10,  5,
    -5,  0,  0,  0,  0,  0,  0, -5,
    -5,  0,  0,  0,  0,  0,  0, -5,
    -5,  0,  0,  0,  0,  0,  0, -5,
    -5,  0,  0,  0,  0,  0,  0, -5,
    -5,  0,  0,  0,  0,  0,  0, -5,
     0,  0,  0,  5,  5,  0,  0,  0,
];

#[rustfmt::skip]
const QUEEN_TABLE: [i32; 64] = [
    -20,-10,-10, -5, -5,-10,-10,-20,
    -10,  0,  0,  0,  0,  0,  0,-10,
    -10,  0,  5,  5,  5,  5,  0,-10,
     -5,  0,  5,  5,  5,  5,  0, -5,
      0,  0,  5,  5,  5,  5,  0, -5,
    -10,  5,  5,  5,  5,  5,  0,-10,
    -10,  0,  5,  0,  0,  0,  0,-10,
    -20,-10,-10, -5, -5,-10,-10,-20,
];

#[rustfmt::skip]
const KING_MIDDLEGAME_TABLE: [i32; 64] = [
    -30,-40,-40,-50,-50,-40,-40,-30,
    -30,-40,-40,-50,-50,-40,-40,-30,
    -30,-40,-40,-50,-50,-40,-40,-30,
    -30,-40,-40,-50,-50,-40,-40,-30,
    -20,-30,-30,-40,-40,-30,-30,-20,
    -10,-20,-20,-20,-20,-20,-20,-10,
     20, 20,  0,  0,  0,  0, 20, 20,
     20, 30, 10,  0,  0, 10, 30, 20,
];

#[rustfmt::skip]
const KING_ENDGAME_TABLE: [i32; 64] = [
    -50,-40,-30,-20,-20,-30,-40,-50,
    -30,-20,-10,  0,  0,-10,-20,-30,
    -30,-10, 20, 30, 30, 20,-10,-30,
    -30,-10, 30, 40, 40, 30,-10,-30,
    -30,-10, 30, 40, 40, 30,-10,-30,
    -30,-10, 20, 30, 30, 20,-10,-30,
    -30,-30,  0,  0,  0,  0,-30,-30,
    -50,-30,-30,-30,-30,-30,-30,-50,
];

/// Non-pawn material of both sides at the start: 4 knights, 4 bishops,
/// 4 rooks and 2 queens. Used to measure how far the game has progressed.
const OPENING_PHASE_MATERIAL: i32 = 4 * 320 + 4 * 330 + 4 * 500 + 2 * 900;

/// Index into a table written from White's point of view.
fn table_index(square: Square, color: Color) -> usize {
    let rank = match color {
        Color::White => 7 - square.rank(),
        Color::Black => square.rank(),
    };
    rank as usize * 8 + square.file() as usize
}

fn piece_table(piece: PieceKind) -> &'static [i32; 64] {
    match piece {
        PieceKind::Pawn => &PAWN_TABLE,
        PieceKind::Knight => &KNIGHT_TABLE,
        PieceKind::Bishop => &BISHOP_TABLE,
        PieceKind::Rook => &ROOK_TABLE,
        PieceKind::Queen => &QUEEN_TABLE,
        PieceKind::King => &KING_MIDDLEGAME_TABLE,
    }
}

/// Squares occupied by `piece` of `color`.
fn squares(board: &Board, piece: PieceKind, color: Color) -> impl Iterator<Item = Square> {
    let mut bits = board.piece_bb(piece).0 & board.color_bb(color).0;
    std::iter::from_fn(move || {
        if bits == 0 {
            return None;
        }
        let index = bits.trailing_zeros() as u8;
        bits &= bits - 1;
        Some(Square::from_index(index))
    })
}

/// Evaluation in centipawns from the point of view of the side to move.
pub fn evaluate(board: &Board) -> i32 {
    let white = evaluate_side(board, Color::White);
    let black = evaluate_side(board, Color::Black);
    match board.side_to_move {
        Color::White => white - black,
        Color::Black => black - white,
    }
}

fn evaluate_side(board: &Board, color: Color) -> i32 {
    let mut score = 0;
    for piece in [
        PieceKind::Pawn,
        PieceKind::Knight,
        PieceKind::Bishop,
        PieceKind::Rook,
        PieceKind::Queen,
    ] {
        let table = piece_table(piece);
        for square in squares(board, piece, color) {
            score += PIECE_VALUES[piece.index()] + table[table_index(square, color)];
        }
    }

    // 256 = middlegame, 0 = endgame.
    let phase = (non_pawn_material(board) * 256 / OPENING_PHASE_MATERIAL).min(256);
    for square in squares(board, PieceKind::King, color) {
        let index = table_index(square, color);
        score += (KING_MIDDLEGAME_TABLE[index] * phase + KING_ENDGAME_TABLE[index] * (256 - phase))
            / 256;
    }
    score
}

fn non_pawn_material(board: &Board) -> i32 {
    [
        PieceKind::Knight,
        PieceKind::Bishop,
        PieceKind::Rook,
        PieceKind::Queen,
    ]
    .into_iter()
    .map(|piece| board.piece_count(piece) as i32 * PIECE_VALUES[piece.index()])
    .sum()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fen::parse_fen;

    fn eval(fen: &str) -> i32 {
        evaluate(&parse_fen(fen).unwrap())
    }

    #[test]
    fn starting_position_is_balanced() {
        assert_eq!(
            eval("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"),
            0
        );
    }

    #[test]
    fn score_is_from_the_side_to_move() {
        let white = eval("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 1");
        let black = eval("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1");
        assert!(white > 0);
        assert_eq!(white, -black);
    }

    #[test]
    fn mirrored_positions_score_the_same() {
        // The same position with colors swapped and the board flipped.
        let white = eval("4k3/8/8/8/3N4/8/PPP5/4K3 w - - 0 1");
        let black = eval("4k3/ppp5/8/3n4/8/8/8/4K3 b - - 0 1");
        assert_eq!(white, black);
    }

    #[test]
    fn material_dominates() {
        // White is a queen up.
        assert!(eval("4k3/8/8/8/8/8/8/3QK3 w - - 0 1") > 800);
        assert!(eval("4k3/8/8/8/8/8/8/3QK3 b - - 0 1") < -800);
    }

    #[test]
    fn centralized_knight_is_better() {
        let center = eval("4k3/8/8/8/3N4/8/8/4K3 w - - 0 1");
        let corner = eval("4k3/8/8/8/8/8/8/N3K3 w - - 0 1");
        assert!(center > corner);
    }

    #[test]
    fn king_centralizes_in_the_endgame() {
        let center = eval("7k/8/8/8/3K4/8/8/8 w - - 0 1");
        let corner = eval("7k/8/8/8/8/8/8/K7 w - - 0 1");
        assert!(center > corner);
    }
}
