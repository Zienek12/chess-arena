mod attacks;
mod board;
mod fen;
mod movegen;
mod moves;
mod perft;
mod status;
mod uci;

use std::io::{self, BufRead, Write};

use uci::UciEngine;

fn main() {
    let stdin = io::stdin();
    let mut stdout = io::stdout();
    let mut engine = UciEngine::new();

    for line in stdin.lock().lines() {
        let line = match line {
            Ok(l) => l,
            Err(_) => break,
        };
        let l = line.trim();
        if l == "quit" {
            break;
        }
        for response in engine.handle_command(l) {
            println!("{}", response);
        }
        stdout.flush().unwrap();
    }
}
