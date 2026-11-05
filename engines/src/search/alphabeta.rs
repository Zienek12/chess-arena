//! Negamax search with alpha-beta pruning, followed by a quiescence search.
//!
//! Two selective techniques search unpromising lines less deeply, at a small
//! risk of missing something: null move pruning and late move reductions.
//!
//! Scores are in centipawns from the side to move. Mate scores encode the
//! distance to mate, so the search prefers the fastest mate and the slowest
//! defeat: being mated `ply` half-moves below the root scores `-(MATE - ply)`.

use crate::board::{Board, PieceKind};
use crate::eval::evaluate;
use crate::hash::position_hash;
use crate::movegen::{in_check, legal_moves};
use crate::moves::Move;
use crate::status::insufficient_material;

use super::heuristics::Heuristics;
use super::ordering::{captured_piece, order_moves, Priorities};
use super::tt::{Bound, TranspositionTable};
use super::{Score, SearchContext};

pub const MATE: i32 = 32_000;
pub const INFINITY: i32 = MATE + 1;
/// Deepest ply a search can reach; mate scores stay above `MATE - MAX_PLY`.
pub const MAX_PLY: u32 = 128;

/// Null move pruning needs enough depth left for its reduced search to mean
/// anything.
const NULL_MOVE_MIN_DEPTH: u32 = 3;
/// Moves are reduced only with this much depth left.
const LMR_MIN_DEPTH: u32 = 3;
/// The moves before this index are searched fully; they are the likely best.
const LMR_FIRST_MOVE: usize = 3;

/// Converts an internal score to the UCI form.
pub fn to_uci_score(score: i32) -> Score {
    if score.abs() > MATE - MAX_PLY as i32 {
        let plies = MATE - score.abs();
        let moves = (plies + 1) / 2;
        Score::Mate(if score > 0 { moves } else { -moves })
    } else {
        Score::Cp(score)
    }
}

/// State shared by every node of one iteration.
pub struct Search<'b, 'c, 'a> {
    pub board: &'b mut Board,
    pub context: &'c mut SearchContext<'a>,
    pub table: &'c mut TranspositionTable,
    pub heuristics: &'c mut Heuristics,
    /// Hashes of the game so far followed by the current search path; the
    /// last entry is the current position.
    pub path: Vec<u64>,
}

impl Search<'_, '_, '_> {
    /// Searches `self.board` to `depth` plies and returns its score. `pv` receives
    /// the best line found. `hint` is the principal variation of the previous
    /// iteration while the search is still following it.
    ///
    /// Returns 0 as soon as the search is aborted; the caller must check
    /// `context.aborted()` and discard the result.
    pub fn negamax(
        &mut self,
        depth: u32,
        ply: u32,
        mut alpha: i32,
        beta: i32,
        hint: Option<&[Move]>,
        pv: &mut Vec<Move>,
    ) -> i32 {
        pv.clear();
        if depth == 0 {
            return self.quiescence(ply, alpha, beta);
        }
        if ply > 0 && self.is_draw() {
            return 0;
        }
        if ply >= MAX_PLY {
            return evaluate(self.board);
        }

        let key = self.path[self.path.len() - 1];
        let entry = self.table.probe(key);
        if let Some(score) = entry.and_then(|entry| entry.cutoff(depth, ply, alpha, beta)) {
            return score;
        }

        let checked = in_check(self.board, self.board.side_to_move);
        if ply > 0 && hint.is_none() && !checked && self.null_move_may_cut(depth, beta) {
            if let Some(score) = self.null_move_search(depth, ply, beta) {
                return score;
            }
        }

        let mut moves = legal_moves(self.board);
        if moves.is_empty() {
            return if checked { -(MATE - ply as i32) } else { 0 };
        }
        let hint_move = hint.and_then(|line| line.first().copied());
        let table_move = entry.and_then(|entry| entry.best_move(&moves));
        let priorities = Priorities {
            hint: hint_move,
            table_move,
            quiet: Some((&*self.heuristics, ply)),
        };
        order_moves(self.board, &mut moves, priorities);

        let original_alpha = alpha;
        let mut best = -INFINITY;
        let mut best_move = moves[0];
        let mut child_pv = Vec::new();
        for (index, &chess_move) in moves.iter().enumerate() {
            let child_hint = match hint {
                Some(line) if Some(chess_move) == hint_move => Some(&line[1..]),
                _ => None,
            };
            let undo = self.board.make_move(chess_move);
            self.context.visit(ply + 1);
            self.path.push(position_hash(self.board));
            // A late quiet move is first searched less deeply, only to see
            // whether it beats alpha. If it does, the reduction was wrong and
            // the move gets the full search.
            let reduction = if depth >= LMR_MIN_DEPTH
                && index >= LMR_FIRST_MOVE
                && !checked
                && chess_move.is_quiet()
                && !in_check(self.board, self.board.side_to_move)
            {
                late_move_reduction(depth, index)
            } else {
                0
            };
            let refuted = reduction > 0
                && -self.negamax(
                    depth - 1 - reduction,
                    ply + 1,
                    -alpha - 1,
                    -alpha,
                    None,
                    &mut child_pv,
                ) <= alpha;
            let score = if refuted {
                alpha
            } else {
                -self.negamax(depth - 1, ply + 1, -beta, -alpha, child_hint, &mut child_pv)
            };
            self.path.pop();
            self.board.unmake_move(chess_move, undo);

            if self.context.poll_stop() {
                return 0;
            }
            if score > best {
                best = score;
                best_move = chess_move;
                if score > alpha {
                    alpha = score;
                    pv.clear();
                    pv.push(chess_move);
                    pv.extend_from_slice(&child_pv);
                }
            }
            if alpha >= beta {
                if chess_move.is_quiet() {
                    let side = self.board.side_to_move;
                    self.heuristics
                        .record_cutoff(side, ply, depth, chess_move, &moves[..index]);
                }
                break;
            }
        }

        let bound = if best >= beta {
            Bound::LOWER
        } else if best <= original_alpha {
            Bound::UPPER
        } else {
            Bound::EXACT
        };
        self.table.store(key, depth, ply, best, bound, best_move);
        best
    }

    /// Whether passing the turn could plausibly still reach `beta`. Without a
    /// piece to move, zugzwang is common and passing could be the only good
    /// "move", so the test would prove nothing.
    ///
    /// This also rules out two passes in a row: the evaluation is the negative
    /// of the opponent's, which was at least its beta, so here it is below beta.
    fn null_move_may_cut(&self, depth: u32, beta: i32) -> bool {
        depth >= NULL_MOVE_MIN_DEPTH
            && beta.abs() <= MATE - MAX_PLY as i32
            && self.board.has_non_pawn_material(self.board.side_to_move)
            && evaluate(self.board) >= beta
    }

    /// Lets the opponent move twice in a row, with a reduced search. If the
    /// score still reaches `beta`, a real move should do so even more, and the
    /// position is cut off at a fraction of the cost. Returns that score.
    fn null_move_search(&mut self, depth: u32, ply: u32, beta: i32) -> Option<i32> {
        let undo = self.board.make_null_move();
        self.context.visit(ply + 1);
        self.path.push(position_hash(self.board));
        let reduced = (depth - 1).saturating_sub(null_move_reduction(depth));
        let score = -self.negamax(reduced, ply + 1, -beta, -beta + 1, None, &mut Vec::new());
        self.path.pop();
        self.board.unmake_null_move(undo);

        if self.context.poll_stop() || score < beta {
            return None;
        }
        // A mate found after passing is not a real line, so its distance means
        // nothing.
        Some(score.min(MATE - MAX_PLY as i32))
    }

    /// Searches on from the horizon until the position is quiet, so that a
    /// position is never evaluated in the middle of an exchange.
    ///
    /// The side to move may "stand pat": decline to capture and accept the
    /// static evaluation, since it nearly always has a quiet move at least that
    /// good. That is a lower bound on its score, so only captures and queen
    /// promotions are tried to beat it. A side in check cannot stand pat and
    /// tries all its moves, which also recognizes checkmate at the horizon.
    ///
    /// Returns 0 as soon as the search is aborted, like [`Self::negamax`].
    fn quiescence(&mut self, ply: u32, mut alpha: i32, beta: i32) -> i32 {
        if self.is_draw() {
            return 0;
        }
        if ply >= MAX_PLY {
            return evaluate(self.board);
        }

        let checked = in_check(self.board, self.board.side_to_move);
        let mut best = -INFINITY;
        if !checked {
            best = evaluate(self.board);
            // Most positions end here, so moves are generated only after this
            // test. A stalemate scoring at least beta goes unnoticed, but a
            // stalemated side is rarely the one ahead.
            if best >= beta {
                return best;
            }
            alpha = alpha.max(best);
        }

        let mut moves = legal_moves(self.board);
        if moves.is_empty() {
            return if checked { -(MATE - ply as i32) } else { 0 };
        }
        if !checked {
            let board = &*self.board;
            moves.retain(|&chess_move| is_tactical(board, chess_move));
        }
        order_moves(self.board, &mut moves, Priorities::default());

        for chess_move in moves {
            let undo = self.board.make_move(chess_move);
            self.context.visit(ply + 1);
            self.path.push(position_hash(self.board));
            let score = -self.quiescence(ply + 1, -beta, -alpha);
            self.path.pop();
            self.board.unmake_move(chess_move, undo);

            if self.context.poll_stop() {
                return 0;
            }
            best = best.max(score);
            alpha = alpha.max(score);
            if alpha >= beta {
                break;
            }
        }
        best
    }

    /// Draws by the fifty-move rule, insufficient material, or repetition. A
    /// single repetition counts: if it was worth repeating once, it is worth
    /// repeating again, so the result is the same as a threefold repetition.
    fn is_draw(&self) -> bool {
        let board = &*self.board;
        if board.halfmove_clock >= 100 || insufficient_material(board) {
            return true;
        }
        let current = self.path[self.path.len() - 1];
        // Positions before the last capture or pawn move cannot repeat.
        self.path
            .iter()
            .rev()
            .skip(1)
            .take(board.halfmove_clock as usize)
            .any(|&hash| hash == current)
    }
}

/// How many plies less a null move is searched, more at greater depths.
fn null_move_reduction(depth: u32) -> u32 {
    2 + depth / 6
}

/// How many plies less a late move is searched: more for later moves and at
/// greater depths, but always leaving at least one ply. Integer logarithms keep
/// the result the same on every platform, so the bench signature is too.
fn late_move_reduction(depth: u32, index: usize) -> u32 {
    let reduction = 1 + depth.ilog2() * (index + 1).ilog2() / 4;
    reduction.min(depth - 2)
}

/// Moves that change the material: captures and queen promotions. Lesser
/// promotions are almost never better and would only enlarge the tree.
fn is_tactical(board: &Board, chess_move: Move) -> bool {
    match chess_move.promotion {
        Some(piece) => piece == PieceKind::Queen,
        None => captured_piece(board, chess_move).is_some(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::fen::parse_fen;
    use crate::search::SearchLimits;
    use std::sync::atomic::AtomicBool;
    use std::sync::Arc;

    /// Runs `body` on a new search of `fen`.
    fn with_search<R>(fen: &str, body: impl FnOnce(&mut Search) -> R) -> R {
        let mut board = parse_fen(fen).unwrap();
        let path = vec![position_hash(&board)];
        let mut on_report = |_: &crate::search::SearchReport| {};
        let mut context = SearchContext::new(
            &board,
            SearchLimits::default(),
            Arc::new(AtomicBool::new(false)),
            &mut on_report,
        );
        let mut table = TranspositionTable::new(1);
        let mut heuristics = Heuristics::new();
        body(&mut Search {
            board: &mut board,
            context: &mut context,
            table: &mut table,
            heuristics: &mut heuristics,
            path,
        })
    }

    /// Quiescence score of `fen` with an open window, and the nodes it visited.
    fn quiesce(fen: &str) -> (i32, u64) {
        with_search(fen, |search| {
            let score = search.quiescence(0, -INFINITY, INFINITY);
            (score, search.context.nodes())
        })
    }

    /// What one search of `fen` at depth 1 with the window `-INFINITY..beta`
    /// learned: the ordering score of `uci` at ply 0.
    fn learned_score(fen: &str, beta: i32, uci: &str) -> i32 {
        with_search(fen, |search| {
            search.negamax(1, 0, -INFINITY, beta, None, &mut Vec::new());
            let chess_move = legal_moves(search.board)
                .into_iter()
                .find(|chess_move| chess_move.to_uci() == uci)
                .unwrap();
            search
                .heuristics
                .quiet_score(search.board.side_to_move, chess_move, 0)
        })
    }

    #[test]
    fn null_move_needs_depth_a_piece_and_a_good_position() {
        let may_cut = |fen, depth, beta| with_search(fen, |s| s.null_move_may_cut(depth, beta));
        // White is a rook up.
        let ahead = "4k3/8/8/8/8/8/8/R3K3 w - - 0 1";
        assert!(may_cut(ahead, 3, 100));
        assert!(!may_cut(ahead, 2, 100));
        assert!(!may_cut(ahead, 3, 900));
        assert!(!may_cut(ahead, 3, MATE - 1));
        // Only pawns: passing could be the way out of a zugzwang.
        assert!(!may_cut("4k3/8/8/8/8/8/P7/4K3 w - - 0 1", 3, -500));
    }

    #[test]
    fn two_passes_in_a_row_are_impossible() {
        // Any beta the first side can pass at leaves the other side's
        // evaluation below its own beta.
        let fen = "4k3/8/8/8/8/8/8/R3K3 w - - 0 1";
        let (eval, first) = with_search(fen, |s| {
            let eval = evaluate(s.board);
            (eval, s.null_move_may_cut(3, eval))
        });
        assert!(first);
        let after_pass = "4k3/8/8/8/8/8/8/R3K3 b - - 0 1";
        assert_eq!(static_eval(after_pass), -eval);
        assert!(!with_search(after_pass, |s| s.null_move_may_cut(3, -eval + 1)));
    }

    #[test]
    fn passing_with_a_winning_position_saves_the_search() {
        // A queen up with a window far below that: the reduced search after
        // passing proves the cutoff.
        let fen = "4k3/pp6/8/8/8/8/PP6/Q3K3 w - - 0 1";
        let (score, cut) = with_search(fen, |s| {
            let cut = s.null_move_search(6, 1, 100);
            (static_eval(fen), cut)
        });
        assert!(score > 100);
        assert!(cut.is_some_and(|cut| cut >= 100));
    }

    #[test]
    fn passing_into_a_mate_threat_does_not_cut_off() {
        // White has three knights against a rook, but if it passes then Black
        // mates with Ra1, as no knight reaches the first rank.
        let fen = "r5k1/8/8/7N/6NN/8/5PPP/6K1 w - - 0 1";
        assert!(static_eval(fen) > 100);
        let cut = with_search(fen, |s| s.null_move_search(5, 1, 100));
        assert_eq!(cut, None);
    }

    #[test]
    fn quiet_cutoff_move_is_remembered() {
        // Ra8# is quiet and refutes any window.
        let fen = "7k/5ppp/8/8/8/8/8/R5K1 w - - 0 1";
        assert!(learned_score(fen, 1_000, "a1a8") > 0);
    }

    #[test]
    fn capture_cutoff_move_is_not_remembered() {
        // Rxd5 wins the queen, which is found without any learning.
        let fen = "4k3/8/8/3q4/8/8/8/3RK3 w - - 0 1";
        assert_eq!(learned_score(fen, 100, "d1d5"), 0);
    }

    #[test]
    fn no_cutoff_teaches_nothing() {
        let fen = "7k/5ppp/8/8/8/8/8/R5K1 w - - 0 1";
        assert_eq!(learned_score(fen, INFINITY, "a1a8"), 0);
    }

    fn static_eval(fen: &str) -> i32 {
        evaluate(&parse_fen(fen).unwrap())
    }

    #[test]
    fn late_moves_are_reduced_more_but_keep_one_ply() {
        assert_eq!(late_move_reduction(3, 3), 1);
        assert_eq!(late_move_reduction(8, 15), 4);
        assert!(late_move_reduction(8, 30) >= late_move_reduction(8, 4));
        assert!(late_move_reduction(16, 10) >= late_move_reduction(4, 10));
        for depth in LMR_MIN_DEPTH..=MAX_PLY {
            for index in LMR_FIRST_MOVE..256 {
                assert!(late_move_reduction(depth, index) <= depth - 2);
            }
        }
    }

    #[test]
    fn null_move_reduction_grows_with_depth() {
        assert_eq!(null_move_reduction(3), 2);
        assert_eq!(null_move_reduction(6), 3);
        assert_eq!(null_move_reduction(12), 4);
    }

    #[test]
    fn quiet_position_scores_the_static_evaluation() {
        let fen = "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1";
        assert_eq!(quiesce(fen), (static_eval(fen), 0));
    }

    #[test]
    fn stands_pat_instead_of_a_losing_capture() {
        // Rxd5 exd5 would give up a rook for a pawn.
        let fen = "4k3/8/4p3/3p4/8/8/8/3RK3 w - - 0 1";
        assert_eq!(quiesce(fen).0, static_eval(fen));
    }

    #[test]
    fn takes_a_winning_capture() {
        let fen = "4k3/8/8/3q4/4P3/8/8/4K3 w - - 0 1";
        let (score, nodes) = quiesce(fen);
        assert!(score > static_eval(fen) + 800, "score {score}");
        assert!(nodes >= 1);
    }

    #[test]
    fn follows_the_whole_exchange() {
        // Rxd5 Rxd5 Rxd5 wins a pawn only because White has the last capture.
        let fen = "3rk3/8/8/3p4/8/8/3R4/3RK3 w - - 0 1";
        assert!(quiesce(fen).0 > static_eval(fen) + 50);
    }

    #[test]
    fn scores_checkmate() {
        assert_eq!(quiesce("7k/6Q1/6K1/8/8/8/8/8 b - - 0 1").0, -MATE);
    }

    #[test]
    fn scores_stalemate_as_a_draw() {
        assert_eq!(quiesce("7k/5Q2/6K1/8/8/8/8/8 b - - 0 1").0, 0);
    }

    #[test]
    fn answers_a_check_with_quiet_moves() {
        // In check, Black cannot stand pat: the only moves are quiet king
        // moves, and all of them are searched.
        let (_, nodes) = quiesce("R3k3/8/8/8/8/8/8/4K3 b - - 0 1");
        assert!(nodes >= 1);
    }

    #[test]
    fn ignores_quiet_moves_and_lesser_promotions() {
        let mut board = parse_fen("4k3/P7/8/8/8/8/8/4K3 w - - 0 1").unwrap();
        let promotions: Vec<String> = legal_moves(&mut board)
            .into_iter()
            .filter(|&chess_move| is_tactical(&board, chess_move))
            .map(|chess_move| chess_move.to_uci())
            .collect();
        assert_eq!(promotions, ["a7a8q"]);
    }

    #[test]
    fn converts_mate_scores_to_moves() {
        assert_eq!(to_uci_score(MATE - 1), Score::Mate(1));
        assert_eq!(to_uci_score(MATE - 3), Score::Mate(2));
        assert_eq!(to_uci_score(-(MATE - 2)), Score::Mate(-1));
        assert_eq!(to_uci_score(-(MATE - 4)), Score::Mate(-2));
        assert_eq!(to_uci_score(125), Score::Cp(125));
        assert_eq!(to_uci_score(-40), Score::Cp(-40));
    }
}
