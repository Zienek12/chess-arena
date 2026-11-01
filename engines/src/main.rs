mod attacks;
mod board;
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
