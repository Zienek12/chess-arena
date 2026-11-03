mod attacks;
mod bench;
mod board;
mod eval;
mod fen;
mod hash;
mod movegen;
mod moves;
mod perft;
mod search;
mod status;
mod uci;

use std::io::{self, BufRead, Write};
use std::sync::Arc;

use uci::{Flow, Output, UciEngine};

fn main() {
    // `chess-arena-engine bench [depth]` runs the benchmark and exits.
    let arguments: Vec<String> = std::env::args().skip(1).collect();
    if arguments.first().map(String::as_str) == Some("bench") {
        match bench::parse_depth(arguments.get(1).map(String::as_str)) {
            Ok(depth) => {
                for line in bench::run(depth).lines() {
                    println!("{}", line);
                }
            }
            Err(error) => {
                eprintln!("{}", error);
                std::process::exit(2);
            }
        }
        return;
    }

    // Each line is written and flushed while stdout is locked, so lines from
    // the search thread never interleave with command responses.
    let output: Output = Arc::new(|line| {
        let mut stdout = io::stdout().lock();
        let _ = writeln!(stdout, "{}", line);
        let _ = stdout.flush();
    });
    let mut engine = UciEngine::new(output);

    for line in io::stdin().lock().lines() {
        let Ok(line) = line else { break };
        if engine.handle_command(line.trim()) == Flow::Quit {
            break;
        }
    }
    // Input closed without quit: finish the search so bestmove is still sent.
    engine.stop_search();
}
