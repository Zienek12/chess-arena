#![allow(dead_code)]

use crate::board::Board;
use crate::fen::parse_fen;
use crate::hash::position_hash;
use crate::movegen::{in_check, legal_moves};
use crate::moves::Move;
use rand::seq::SliceRandom;
use std::time::{Duration, Instant};

const START_FEN: &str = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1";

pub struct UciEngine {
    pub board: Board,
    history: Vec<u64>,
}

impl UciEngine {
    pub fn new() -> Self {
        let board = parse_fen(START_FEN).expect("start FEN must be valid");
        Self {
            history: vec![position_hash(&board)],
            board,
        }
    }

    pub fn handle_command(&mut self, command: &str) -> Vec<String> {
        let mut parts = command.split_whitespace();
        let Some(keyword) = parts.next() else {
            return Vec::new();
        };

        match keyword {
            "uci" => vec![
                "id name uci_stub".to_string(),
                "id author example".to_string(),
                "uciok".to_string(),
            ],
            "isready" => vec!["readyok".to_string()],
            "ucinewgame" => {
                self.board = parse_fen(START_FEN).expect("start FEN must be valid");
                self.history = vec![position_hash(&self.board)];
                Vec::new()
            }
            "position" => match self.set_position(parts.collect()) {
                Ok(()) => Vec::new(),
                Err(error) => vec![format!("info string error {}", error)],
            },
            "go" => self.bestmove(),
            "d" => vec![format!("{}", self.board)],
            "stop" => Vec::new(),
            "quit" => Vec::new(),
            _ => vec![format!("info string error unknown command {}", keyword)],
        }
    }

    fn set_position(&mut self, arguments: Vec<&str>) -> Result<(), String> {
        if arguments.is_empty() {
            return Err("missing position type".to_string());
        }
        let (fen, move_start) = match arguments[0] {
            "startpos" => (START_FEN.to_string(), 1),
            "fen" => {
                if arguments.len() < 7 {
                    return Err("incomplete fen position".to_string());
                }
                (arguments[1..7].join(" "), 7)
            }
            _ => return Err("unknown position type".to_string()),
        };

        let mut board = parse_fen(&fen).map_err(|error| error.to_string())?;
        let mut history = vec![position_hash(&board)];
        let moves = if move_start < arguments.len() && arguments[move_start] == "moves" {
            &arguments[move_start + 1..]
        } else {
            &[]
        };
        for notation in moves {
            let chess_move = find_uci_move(&mut board, notation)
                .ok_or_else(|| format!("illegal move {}", notation))?;
            board.make_move(chess_move);
            history.push(position_hash(&board));
        }
        self.board = board;
        self.history = history;
        Ok(())
    }

    fn bestmove(&mut self) -> Vec<String> {
        let started = Instant::now();
        let moves = legal_moves(&mut self.board);
        let Some(chess_move) = moves.choose(&mut rand::thread_rng()) else {
            // No legal moves: report the terminal score so the arena sees mate or stalemate.
            let score = if in_check(&self.board, self.board.side_to_move) {
                "mate 0"
            } else {
                "cp 0"
            };
            return vec![
                format!("info depth 0 score {}", score),
                "bestmove 0000".to_string(),
            ];
        };

        // Until a real search exists, every legal move counts as one searched node at depth 1.
        let telemetry = SearchTelemetry {
            depth: 1,
            score_cp: 0,
            nodes: moves.len() as u64,
            elapsed: started.elapsed(),
            pv: vec![*chess_move],
        };
        vec![
            telemetry.to_info_line(),
            format!("bestmove {}", chess_move.to_uci()),
        ]
    }
}

struct SearchTelemetry {
    depth: u32,
    score_cp: i32,
    nodes: u64,
    elapsed: Duration,
    pv: Vec<Move>,
}

impl SearchTelemetry {
    fn to_info_line(&self) -> String {
        let micros = self.elapsed.as_micros().max(1);
        let nps = (u128::from(self.nodes) * 1_000_000 / micros) as u64;
        let pv: Vec<String> = self
            .pv
            .iter()
            .map(|chess_move| chess_move.to_uci())
            .collect();
        format!(
            "info depth {} seldepth {} score cp {} nodes {} nps {} time {} pv {}",
            self.depth,
            self.depth,
            self.score_cp,
            self.nodes,
            nps,
            self.elapsed.as_millis(),
            pv.join(" ")
        )
    }
}

fn find_uci_move(board: &mut Board, notation: &str) -> Option<Move> {
    let moves = legal_moves(board);
    moves
        .into_iter()
        .find(|chess_move| chess_move.to_uci() == notation)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::{Color, PieceKind, Square};
    use crate::status::game_status_with_history;

    #[test]
    fn handles_start_position_and_go() {
        let mut engine = UciEngine::new();
        assert_eq!(
            engine.handle_command("position startpos moves e2e4"),
            Vec::<String>::new()
        );
        assert_eq!(engine.board.side_to_move, Color::Black);
        let response = engine.handle_command("go");
        assert_eq!(response.len(), 2);
        let notation = response[1].strip_prefix("bestmove ").unwrap();
        assert!(notation.len() == 4);
        assert!(legal_moves(&mut engine.board)
            .iter()
            .any(|chess_move| chess_move.to_uci() == notation));
    }

    #[test]
    fn handles_fen_and_ready() {
        let mut engine = UciEngine::new();
        let result = engine.handle_command("position fen 8/8/8/8/8/8/4K3/7k w - - 0 1");
        assert!(result.is_empty());
        assert_eq!(engine.handle_command("isready"), vec!["readyok"]);
    }

    #[test]
    fn reports_errors_with_error_prefix() {
        let mut engine = UciEngine::new();
        assert_eq!(
            engine.handle_command("position startpos moves e2e5"),
            vec!["info string error illegal move e2e5"]
        );
        assert_eq!(
            engine.handle_command("foo"),
            vec!["info string error unknown command foo"]
        );
    }

    #[test]
    fn reports_telemetry_before_bestmove() {
        let mut engine = UciEngine::new();
        let response = engine.handle_command("go");
        let fields: Vec<&str> = response[0].split_whitespace().collect();
        assert_eq!(
            &fields[..7],
            ["info", "depth", "1", "seldepth", "1", "score", "cp"]
        );
        assert_eq!(fields[8..10], ["nodes", "20"]);
        assert!(fields.contains(&"nps") && fields.contains(&"time"));
        let pv = fields.last().unwrap();
        assert_eq!(response[1], format!("bestmove {}", pv));
    }

    #[test]
    fn reports_mate_score_when_checkmated() {
        let mut engine = UciEngine::new();
        engine.handle_command("position startpos moves f2f3 e7e5 g2g4 d8h4");
        assert_eq!(
            engine.handle_command("go"),
            vec!["info depth 0 score mate 0", "bestmove 0000"]
        );
    }

    #[test]
    fn reports_draw_score_when_stalemated() {
        let mut engine = UciEngine::new();
        engine.handle_command("position fen 7k/5Q2/6K1/8/8/8/8/8 b - - 0 1");
        assert_eq!(
            engine.handle_command("go"),
            vec!["info depth 0 score cp 0", "bestmove 0000"]
        );
    }

    #[test]
    fn serializes_promotion_move() {
        let chess_move = Move::promotion(Square::E7, Square::E8, PieceKind::Queen, false);
        assert_eq!(chess_move.to_uci(), "e7e8q");
    }

    #[test]
    fn tracks_threefold_repetition_history() {
        let mut engine = UciEngine::new();
        engine.handle_command("position startpos moves g1f3 g8f6 f3g1 f6g8 g1f3 g8f6 f3g1 f6g8");
        assert_eq!(
            game_status_with_history(&mut engine.board, &engine.history),
            crate::status::GameStatus::ThreefoldRepetition
        );
    }
}
