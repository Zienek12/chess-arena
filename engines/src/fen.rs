#![allow(dead_code)]

use std::fmt;

use crate::board::{Board, CastlingRights, Color, PieceKind, Square};

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct FenError(pub String);

impl fmt::Display for FenError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for FenError {}

pub fn parse_fen(fen: &str) -> Result<Board, FenError> {
    let fields: Vec<&str> = fen.split_whitespace().collect();
    if fields.len() != 6 {
        return Err(FenError("fen must contain six fields".to_string()));
    }

    let mut board = Board::empty();
    parse_placement(fields[0], &mut board)?;

    board.side_to_move = match fields[1] {
        "w" => Color::White,
        "b" => Color::Black,
        _ => return Err(FenError("invalid side to move".to_string())),
    };
    board.castling_rights = parse_castling(fields[2])?;
    board.en_passant = parse_en_passant(fields[3])?;
    board.halfmove_clock = parse_number(fields[4], "halfmove clock")?;
    board.fullmove_number = parse_number(fields[5], "fullmove number")?;
    if board.fullmove_number == 0 {
        return Err(FenError(
            "fullmove number must be greater than zero".to_string(),
        ));
    }
    Ok(board)
}

pub fn board_to_fen(board: &Board) -> String {
    let mut placement = String::new();
    for rank in (0u8..8).rev() {
        let mut empty = 0u8;
        for file in 0u8..8 {
            let square = Square::from_coords(file, rank);
            match board.piece_at(square) {
                Some((piece, color)) => {
                    if empty != 0 {
                        placement.push(char::from(b'0' + empty));
                        empty = 0;
                    }
                    placement.push(piece_char(piece, color));
                }
                None => empty += 1,
            }
        }
        if empty != 0 {
            placement.push(char::from(b'0' + empty));
        }
        if rank != 0 {
            placement.push('/');
        }
    }

    let side = match board.side_to_move {
        Color::White => 'w',
        Color::Black => 'b',
    };
    let castling = castling_text(board.castling_rights);
    let en_passant = board
        .en_passant
        .map(square_name)
        .unwrap_or_else(|| "-".to_string());
    format!(
        "{} {} {} {} {} {}",
        placement, side, castling, en_passant, board.halfmove_clock, board.fullmove_number
    )
}

fn parse_placement(placement: &str, board: &mut Board) -> Result<(), FenError> {
    let ranks: Vec<&str> = placement.split('/').collect();
    if ranks.len() != 8 {
        return Err(FenError(
            "piece placement must contain eight ranks".to_string(),
        ));
    }

    for (rank_index, rank_text) in ranks.iter().enumerate() {
        let rank = 7 - rank_index as u8;
        let mut file = 0u8;
        for character in rank_text.chars() {
            if character.is_ascii_digit() {
                let empty = character.to_digit(10).unwrap() as u8;
                if empty == 0 || empty > 8 || file + empty > 8 {
                    return Err(FenError("invalid empty-square count".to_string()));
                }
                file += empty;
            } else if let Some((piece, color)) = piece_from_char(character) {
                if file >= 8 {
                    return Err(FenError("rank contains too many squares".to_string()));
                }
                board.set_piece(Square::from_coords(file, rank), piece, color);
                file += 1;
            } else {
                return Err(FenError("invalid piece character".to_string()));
            }
        }
        if file != 8 {
            return Err(FenError("rank does not contain eight squares".to_string()));
        }
    }
    Ok(())
}

fn parse_castling(value: &str) -> Result<CastlingRights, FenError> {
    if value == "-" {
        return Ok(CastlingRights::NONE);
    }
    let mut rights = CastlingRights::NONE;
    for character in value.chars() {
        let flag = match character {
            'K' => CastlingRights::WHITE_KING_SIDE,
            'Q' => CastlingRights::WHITE_QUEEN_SIDE,
            'k' => CastlingRights::BLACK_KING_SIDE,
            'q' => CastlingRights::BLACK_QUEEN_SIDE,
            _ => return Err(FenError("invalid castling rights".to_string())),
        };
        if rights.contains(flag) {
            return Err(FenError("duplicate castling right".to_string()));
        }
        rights = CastlingRights::new(rights.bits() | flag.bits());
    }
    Ok(rights)
}

fn parse_en_passant(value: &str) -> Result<Option<Square>, FenError> {
    if value == "-" {
        return Ok(None);
    }
    let bytes = value.as_bytes();
    if bytes.len() != 2 || !(b'a'..=b'h').contains(&bytes[0]) || !(b'1'..=b'8').contains(&bytes[1])
    {
        return Err(FenError("invalid en passant square".to_string()));
    }
    Ok(Some(Square::from_coords(bytes[0] - b'a', bytes[1] - b'1')))
}

fn parse_number(value: &str, name: &str) -> Result<u16, FenError> {
    value
        .parse::<u16>()
        .map_err(|_| FenError(format!("invalid {}", name)))
}

fn piece_from_char(character: char) -> Option<(PieceKind, Color)> {
    let color = if character.is_ascii_uppercase() {
        Color::White
    } else {
        Color::Black
    };
    let piece = match character.to_ascii_lowercase() {
        'p' => PieceKind::Pawn,
        'n' => PieceKind::Knight,
        'b' => PieceKind::Bishop,
        'r' => PieceKind::Rook,
        'q' => PieceKind::Queen,
        'k' => PieceKind::King,
        _ => return None,
    };
    Some((piece, color))
}

fn piece_char(piece: PieceKind, color: Color) -> char {
    let character = match piece {
        PieceKind::Pawn => 'p',
        PieceKind::Knight => 'n',
        PieceKind::Bishop => 'b',
        PieceKind::Rook => 'r',
        PieceKind::Queen => 'q',
        PieceKind::King => 'k',
    };
    if color == Color::White {
        character.to_ascii_uppercase()
    } else {
        character
    }
}

fn castling_text(rights: CastlingRights) -> String {
    let mut value = String::new();
    for (flag, character) in [
        (CastlingRights::WHITE_KING_SIDE, 'K'),
        (CastlingRights::WHITE_QUEEN_SIDE, 'Q'),
        (CastlingRights::BLACK_KING_SIDE, 'k'),
        (CastlingRights::BLACK_QUEEN_SIDE, 'q'),
    ] {
        if rights.contains(flag) {
            value.push(character);
        }
    }
    if value.is_empty() {
        value.push('-');
    }
    value
}

fn square_name(square: Square) -> String {
    let mut value = String::with_capacity(2);
    value.push(char::from(b'a' + square.file()));
    value.push(char::from(b'1' + square.rank()));
    value
}

#[cfg(test)]
mod tests {
    use super::*;

    const START: &str = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1";

    #[test]
    fn start_position_round_trips() {
        let board = parse_fen(START).unwrap();
        assert_eq!(board_to_fen(&board), START);
    }

    #[test]
    fn parses_state_fields() {
        let fen = "8/8/8/3pP3/8/8/8/4K2k b Qq e6 17 42";
        let board = parse_fen(fen).unwrap();
        assert_eq!(board.side_to_move, Color::Black);
        assert_eq!(board.en_passant, Some(Square::E6));
        assert_eq!(board.halfmove_clock, 17);
        assert_eq!(board.fullmove_number, 42);
        assert_eq!(board_to_fen(&board), fen);
    }

    #[test]
    fn rejects_invalid_rank() {
        assert!(parse_fen("8/8/8/8/8/8/8/9 w - - 0 1").is_err());
    }
}
