use std::io::{self, BufRead, Write};

fn main() {
    let stdin = io::stdin();
    let mut stdout = io::stdout();

    for line in stdin.lock().lines() {
        let line = match line { Ok(l) => l, Err(_) => break };
        let l = line.trim();
        if l == "uci" {
            println!("id name uci_stub");
            println!("id author example");
            println!("uciok");
            stdout.flush().unwrap();
        } else if l == "isready" {
            println!("readyok");
            stdout.flush().unwrap();
        } else if l.starts_with("position") {
            // ignore for stub
        } else if l.starts_with("go") {
            // respond with a dummy bestmove
            println!("bestmove e2e4");
            stdout.flush().unwrap();
        } else if l == "quit" {
            break;
        }
    }
}
