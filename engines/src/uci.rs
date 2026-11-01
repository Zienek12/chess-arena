#![allow(dead_code)]

use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use crate::board::Board;
use crate::fen::parse_fen;
use crate::hash::position_hash;
use crate::movegen::legal_moves;
use crate::moves::Move;
use crate::search::{search, SearchLimits};

const START_FEN: &str = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1";
// Reported with the crate version, which matches the engine-vX.Y.Z release tag.
const ENGINE_NAME: &str = "chess-arena-engine";

/// Receives every line the engine sends to the GUI. The search thread uses it
/// too, so lines are written as soon as they exist.
pub type Output = Arc<dyn Fn(String) + Send + Sync>;

/// Whether the command loop should keep reading commands.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Flow {
    Continue,
    Quit,
}

struct RunningSearch {
    stop: Arc<AtomicBool>,
    thread: JoinHandle<()>,
}

pub struct UciEngine {
    pub board: Board,
    history: Vec<u64>,
    output: Output,
    search: Option<RunningSearch>,
}

impl UciEngine {
    pub fn new(output: Output) -> Self {
        let board = parse_fen(START_FEN).expect("start FEN must be valid");
        Self {
            history: vec![position_hash(&board)],
            board,
            output,
            search: None,
        }
    }

    fn send(&self, line: impl Into<String>) {
        (self.output)(line.into());
    }

    pub fn handle_command(&mut self, command: &str) -> Flow {
        let mut parts = command.split_whitespace();
        let Some(keyword) = parts.next() else {
            return Flow::Continue;
        };

        match keyword {
            "uci" => {
                self.send(format!(
                    "id name {} {}",
                    ENGINE_NAME,
                    env!("CARGO_PKG_VERSION")
                ));
                self.send("id author Chess Arena");
                self.send("uciok");
            }
            // Answered at once, even while a search is running.
            "isready" => self.send("readyok"),
            "ucinewgame" => {
                self.stop_search();
                self.board = parse_fen(START_FEN).expect("start FEN must be valid");
                self.history = vec![position_hash(&self.board)];
            }
            "position" => {
                self.stop_search();
                if let Err(error) = self.set_position(parts.collect()) {
                    self.send(format!("info string error {}", error));
                }
            }
            "go" => match SearchLimits::parse(&parts.collect::<Vec<_>>()) {
                Ok(limits) => self.start_search(limits),
                Err(error) => self.send(format!("info string error {}", error)),
            },
            "stop" => self.stop_search(),
            "d" => self.send(format!("{}", self.board)),
            "quit" => {
                self.stop_search();
                return Flow::Quit;
            }
            _ => self.send(format!("info string error unknown command {}", keyword)),
        }
        Flow::Continue
    }

    /// Starts searching the current position on its own thread. Telemetry is
    /// sent after every iteration, then `bestmove`.
    fn start_search(&mut self, limits: SearchLimits) {
        self.stop_search();
        let stop = Arc::new(AtomicBool::new(false));
        let thread_stop = Arc::clone(&stop);
        let output = Arc::clone(&self.output);
        let mut board = self.board;
        let history = self.history.clone();
        let thread = thread::spawn(move || {
            let infinite = limits.infinite;
            let result = search(
                &mut board,
                &history,
                limits,
                Arc::clone(&thread_stop),
                &mut |report| output(report.to_info_line()),
            );
            // UCI forbids bestmove before stop when searching infinitely.
            while infinite && !thread_stop.load(Ordering::Relaxed) {
                thread::sleep(Duration::from_millis(1));
            }
            let best = result
                .best_move
                .map_or_else(|| "0000".to_string(), |chess_move| chess_move.to_uci());
            output(format!("bestmove {}", best));
        });
        self.search = Some(RunningSearch { stop, thread });
    }

    /// Stops a running search and waits for its bestmove.
    pub fn stop_search(&mut self) {
        if let Some(running) = self.search.take() {
            running.stop.store(true, Ordering::Relaxed);
            let _ = running.thread.join();
        }
    }

    /// Waits for a running search to finish on its own.
    pub fn wait_for_search(&mut self) {
        if let Some(running) = self.search.take() {
            let _ = running.thread.join();
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
    use std::sync::Mutex;

    /// An engine whose output is captured for assertions.
    struct Harness {
        engine: UciEngine,
        lines: Arc<Mutex<Vec<String>>>,
    }

    impl Harness {
        fn new() -> Self {
            let lines = Arc::new(Mutex::new(Vec::new()));
            let sink = Arc::clone(&lines);
            let output: Output = Arc::new(move |line| sink.lock().unwrap().push(line));
            Self {
                engine: UciEngine::new(output),
                lines,
            }
        }

        /// Sends a command, waits for any search it started, and returns its output.
        fn run(&mut self, command: &str) -> Vec<String> {
            self.lines.lock().unwrap().clear();
            self.engine.handle_command(command);
            self.engine.wait_for_search();
            std::mem::take(&mut *self.lines.lock().unwrap())
        }

        fn output(&self) -> Vec<String> {
            self.lines.lock().unwrap().clone()
        }
    }

    #[test]
    fn handles_start_position_and_go() {
        let mut uci = Harness::new();
        assert!(uci.run("position startpos moves e2e4").is_empty());
        assert_eq!(uci.engine.board.side_to_move, Color::Black);
        let response = uci.run("go movetime 100");
        // One info line per completed depth, then bestmove.
        let (bestmove, infos) = response.split_last().unwrap();
        assert!(!infos.is_empty());
        assert!(infos.iter().all(|line| line.starts_with("info depth ")));
        let notation = bestmove.strip_prefix("bestmove ").unwrap();
        assert!(legal_moves(&mut uci.engine.board)
            .iter()
            .any(|chess_move| chess_move.to_uci() == notation));
    }

    #[test]
    fn handles_fen_and_ready() {
        let mut uci = Harness::new();
        assert!(uci
            .run("position fen 8/8/8/8/8/8/4K3/7k w - - 0 1")
            .is_empty());
        assert_eq!(uci.run("isready"), vec!["readyok"]);
    }

    #[test]
    fn reports_errors_with_error_prefix() {
        let mut uci = Harness::new();
        assert_eq!(
            uci.run("position startpos moves e2e5"),
            vec!["info string error illegal move e2e5"]
        );
        assert_eq!(
            uci.run("foo"),
            vec!["info string error unknown command foo"]
        );
        assert_eq!(
            uci.run("go movetime soon"),
            vec!["info string error invalid go movetime value soon"]
        );
    }

    #[test]
    fn reports_telemetry_before_bestmove() {
        let mut uci = Harness::new();
        let response = uci.run("go depth 1");
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
        let mut uci = Harness::new();
        uci.run("position startpos moves f2f3 e7e5 g2g4 d8h4");
        assert_eq!(
            uci.run("go"),
            vec![
                "info depth 0 seldepth 0 score mate 0 nodes 0 nps 0 time 0",
                "bestmove 0000"
            ]
        );
    }

    #[test]
    fn reports_draw_score_when_stalemated() {
        let mut uci = Harness::new();
        uci.run("position fen 7k/5Q2/6K1/8/8/8/8/8 b - - 0 1");
        assert_eq!(
            uci.run("go"),
            vec![
                "info depth 0 seldepth 0 score cp 0 nodes 0 nps 0 time 0",
                "bestmove 0000"
            ]
        );
    }

    #[test]
    fn infinite_search_waits_for_stop() {
        let mut uci = Harness::new();
        uci.engine.handle_command("go infinite");
        thread::sleep(Duration::from_millis(20));
        assert!(uci
            .output()
            .iter()
            .all(|line| !line.starts_with("bestmove")));
        uci.engine.handle_command("isready");
        assert_eq!(uci.output().last().unwrap(), "readyok");
        uci.engine.handle_command("stop");
        assert!(uci.output().last().unwrap().starts_with("bestmove "));
    }

    #[test]
    fn quit_stops_a_running_search() {
        let mut uci = Harness::new();
        uci.engine.handle_command("go infinite");
        assert_eq!(uci.engine.handle_command("quit"), Flow::Quit);
        assert!(uci.output().last().unwrap().starts_with("bestmove "));
    }

    #[test]
    fn reports_name_with_crate_version() {
        let mut uci = Harness::new();
        let response = uci.run("uci");
        assert_eq!(
            response[0],
            format!("id name chess-arena-engine {}", env!("CARGO_PKG_VERSION"))
        );
        assert_eq!(response.last().unwrap(), "uciok");
    }

    #[test]
    fn serializes_promotion_move() {
        let chess_move = Move::promotion(Square::E7, Square::E8, PieceKind::Queen, false);
        assert_eq!(chess_move.to_uci(), "e7e8q");
    }

    #[test]
    fn tracks_threefold_repetition_history() {
        let mut uci = Harness::new();
        uci.run("position startpos moves g1f3 g8f6 f3g1 f6g8 g1f3 g8f6 f3g1 f6g8");
        assert_eq!(
            game_status_with_history(&mut uci.engine.board, &uci.engine.history),
            crate::status::GameStatus::ThreefoldRepetition
        );
    }
}
