//! Transposition table: remembers what was found about searched positions.
//!
//! The same position is often reached by different move orders, and again at
//! the next iteration. A stored score can answer such a position at once if
//! it was searched at least as deep, and a stored best move is a strong first
//! move to try even when it was not.
//!
//! The table has one slot per index, chosen by the low bits of the position
//! hash. A slot keeps the whole hash to recognize its position.

use crate::board::PieceKind;
use crate::moves::Move;

use super::alphabeta::{MATE, MAX_PLY};

pub const DEFAULT_SIZE_MIB: usize = 64;
pub const MIN_SIZE_MIB: usize = 1;
pub const MAX_SIZE_MIB: usize = 1024;

const SLOT_BYTES: usize = 16;

/// What a stored score says about the true score of the position. A search
/// that stays inside its window knows it exactly, which is both bounds at once.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Bound(u8);

impl Bound {
    /// All moves failed low: the true score is at most the stored one.
    pub const UPPER: Self = Self(0b01);
    /// A move failed high: the true score is at least the stored one.
    pub const LOWER: Self = Self(0b10);
    pub const EXACT: Self = Self(0b11);

    fn is_upper(self) -> bool {
        self.0 & Self::UPPER.0 != 0
    }

    fn is_lower(self) -> bool {
        self.0 & Self::LOWER.0 != 0
    }
}

/// A stored search result, with the score relative to the stored node.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Entry {
    depth: u32,
    score: i32,
    bound: Bound,
    move_code: u16,
}

impl Entry {
    /// The score if this entry was searched at least to `depth` and proves
    /// that the node fails outside the window `alpha..beta`, so its moves need
    /// not be searched. A score inside the window is not used: such a node is
    /// on the principal variation, and searching it keeps that line complete.
    pub fn cutoff(&self, depth: u32, ply: u32, alpha: i32, beta: i32) -> Option<i32> {
        if self.depth < depth {
            return None;
        }
        let score = score_from_table(self.score, ply);
        let fails =
            (self.bound.is_upper() && score <= alpha) || (self.bound.is_lower() && score >= beta);
        fails.then_some(score)
    }

    /// Finds the stored best move in `moves`. A hash collision can only give
    /// a code that matches no move, or a legal move that is merely tried first.
    pub fn best_move(&self, moves: &[Move]) -> Option<Move> {
        moves
            .iter()
            .copied()
            .find(|&chess_move| move_code(chess_move) == self.move_code)
    }
}

pub struct TranspositionTable {
    /// Each slot is the position hash and the packed entry. All zeros is an
    /// empty slot, so the memory comes zeroed from the system and costs nothing
    /// until it is first written.
    slots: Vec<[u64; 2]>,
}

impl TranspositionTable {
    /// A table of at most `size_mib` mebibytes; the slot count is a power of
    /// two, so an index is a bit mask of the hash.
    pub fn new(size_mib: usize) -> Self {
        let wanted = size_mib.max(MIN_SIZE_MIB) * 1024 * 1024 / SLOT_BYTES;
        let slots = 1usize << wanted.ilog2();
        Self {
            slots: vec![[0; 2]; slots],
        }
    }

    pub fn clear(&mut self) {
        // A new allocation is zeroed lazily, unlike writing every slot.
        self.slots = vec![[0; 2]; self.slots.len()];
    }

    fn index(&self, key: u64) -> usize {
        key as usize & (self.slots.len() - 1)
    }

    pub fn probe(&self, key: u64) -> Option<Entry> {
        let [stored_key, data] = self.slots[self.index(key)];
        (stored_key == key && data != 0).then(|| unpack(data))
    }

    /// Stores a result, with `score` relative to the root as the search uses
    /// it. It replaces the slot unless the slot holds a deeper result for the
    /// same position.
    pub fn store(&mut self, key: u64, depth: u32, ply: u32, score: i32, bound: Bound, best: Move) {
        let index = self.index(key);
        let [stored_key, data] = self.slots[index];
        if stored_key == key && data != 0 && unpack(data).depth > depth {
            return;
        }
        self.slots[index] = [
            key,
            pack(Entry {
                depth,
                score: score_to_table(score, ply),
                bound,
                move_code: move_code(best),
            }),
        ];
    }
}

/// A mate score counts plies from the root, but the same position can occur at
/// any ply. The table stores it counted from the node itself.
fn score_to_table(score: i32, ply: u32) -> i32 {
    if score > MATE - MAX_PLY as i32 {
        score + ply as i32
    } else if score < -(MATE - MAX_PLY as i32) {
        score - ply as i32
    } else {
        score
    }
}

fn score_from_table(score: i32, ply: u32) -> i32 {
    if score > MATE - MAX_PLY as i32 {
        score - ply as i32
    } else if score < -(MATE - MAX_PLY as i32) {
        score + ply as i32
    } else {
        score
    }
}

/// The squares and promotion piece, which identify a move in a position. It is
/// never 0, because the two squares of a move differ.
fn move_code(chess_move: Move) -> u16 {
    let promotion = match chess_move.promotion {
        None => 0,
        Some(PieceKind::Knight) => 1,
        Some(PieceKind::Bishop) => 2,
        Some(PieceKind::Rook) => 3,
        Some(_) => 4,
    };
    chess_move.from.index() as u16 | (chess_move.to.index() as u16) << 6 | promotion << 12
}

fn pack(entry: Entry) -> u64 {
    (entry.score as i16 as u16 as u64)
        | (entry.depth as u64) << 16
        | (entry.bound.0 as u64) << 24
        | (entry.move_code as u64) << 32
}

fn unpack(data: u64) -> Entry {
    Entry {
        score: data as u16 as i16 as i32,
        depth: (data >> 16) as u8 as u32,
        bound: Bound((data >> 24) as u8),
        move_code: (data >> 32) as u16,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::board::Square;
    use crate::moves::MoveKind;

    const KEY: u64 = 0x1234_5678_9abc_def0;

    fn table() -> TranspositionTable {
        TranspositionTable::new(1)
    }

    fn e2e4() -> Move {
        Move::new(Square::E2, Square::E4, MoveKind::DoublePawnPush)
    }

    #[test]
    fn slot_count_is_a_power_of_two_within_the_size() {
        assert_eq!(table().slots.len(), 1 << 16);
        assert_eq!(TranspositionTable::new(3).slots.len(), 1 << 17);
    }

    #[test]
    fn empty_table_finds_nothing() {
        assert_eq!(table().probe(KEY), None);
        assert_eq!(table().probe(0), None);
    }

    #[test]
    fn stored_entry_is_found_with_its_values() {
        let mut table = table();
        table.store(KEY, 7, 3, -1234, Bound::EXACT, e2e4());
        let entry = table.probe(KEY).unwrap();
        assert_eq!(entry.depth, 7);
        assert_eq!(entry.score, -1234);
        assert_eq!(entry.bound, Bound::EXACT);
        assert_eq!(entry.best_move(&[e2e4()]), Some(e2e4()));
    }

    #[test]
    fn a_different_position_in_the_slot_is_not_used() {
        let mut table = table();
        table.store(KEY, 7, 0, 10, Bound::EXACT, e2e4());
        // The same low bits select the same slot, the whole hash differs.
        assert_eq!(table.probe(KEY ^ (1 << 63)), None);
    }

    #[test]
    fn deeper_result_for_the_same_position_is_kept() {
        let mut table = table();
        table.store(KEY, 7, 0, 10, Bound::EXACT, e2e4());
        table.store(KEY, 3, 0, 20, Bound::EXACT, e2e4());
        assert_eq!(table.probe(KEY).unwrap().score, 10);
        table.store(KEY, 7, 0, 30, Bound::EXACT, e2e4());
        assert_eq!(table.probe(KEY).unwrap().score, 30);
    }

    #[test]
    fn another_position_replaces_the_slot() {
        let mut table = table();
        let other = KEY ^ (1 << 63);
        table.store(KEY, 7, 0, 10, Bound::EXACT, e2e4());
        table.store(other, 1, 0, 20, Bound::EXACT, e2e4());
        assert_eq!(table.probe(KEY), None);
        assert_eq!(table.probe(other).unwrap().score, 20);
    }

    #[test]
    fn clear_empties_the_table() {
        let mut table = table();
        table.store(KEY, 7, 0, 10, Bound::EXACT, e2e4());
        table.clear();
        assert_eq!(table.probe(KEY), None);
    }

    fn entry(depth: u32, score: i32, bound: Bound) -> Entry {
        Entry {
            depth,
            score,
            bound,
            move_code: 1,
        }
    }

    #[test]
    fn shallower_result_gives_no_cutoff() {
        assert_eq!(entry(3, 500, Bound::EXACT).cutoff(4, 0, 0, 100), None);
    }

    #[test]
    fn bounds_cut_off_only_on_their_side() {
        assert_eq!(entry(5, 500, Bound::LOWER).cutoff(4, 0, 0, 100), Some(500));
        assert_eq!(entry(5, 50, Bound::LOWER).cutoff(4, 0, 0, 100), None);
        assert_eq!(entry(5, -50, Bound::UPPER).cutoff(4, 0, 0, 100), Some(-50));
        assert_eq!(entry(5, 50, Bound::UPPER).cutoff(4, 0, 0, 100), None);
    }

    #[test]
    fn exact_score_inside_the_window_is_searched_again() {
        assert_eq!(entry(5, 50, Bound::EXACT).cutoff(4, 0, 0, 100), None);
        assert_eq!(entry(5, 150, Bound::EXACT).cutoff(4, 0, 0, 100), Some(150));
        assert_eq!(entry(5, -50, Bound::EXACT).cutoff(4, 0, 0, 100), Some(-50));
    }

    #[test]
    fn mate_scores_follow_the_ply_of_the_position() {
        let mut table = table();
        // Mate 5 plies below a node at ply 4, found while searching that node.
        table.store(KEY, 7, 4, MATE - 9, Bound::EXACT, e2e4());
        let entry = table.probe(KEY).unwrap();
        // The same position at ply 2 is mated two plies nearer to the root.
        assert_eq!(entry.cutoff(1, 2, 0, 100), Some(MATE - 7));

        table.store(KEY, 8, 4, -(MATE - 9), Bound::EXACT, e2e4());
        let entry = table.probe(KEY).unwrap();
        assert_eq!(entry.cutoff(1, 2, 0, 100), Some(-(MATE - 7)));
    }

    #[test]
    fn promotions_to_different_pieces_are_different_moves() {
        let queen = Move::promotion(Square::E7, Square::E8, PieceKind::Queen, false);
        let knight = Move::promotion(Square::E7, Square::E8, PieceKind::Knight, false);
        let mut table = table();
        table.store(KEY, 1, 0, 0, Bound::EXACT, knight);
        let entry = table.probe(KEY).unwrap();
        assert_eq!(entry.best_move(&[queen, knight]), Some(knight));
    }
}
