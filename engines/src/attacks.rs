#![allow(dead_code)]

//! Attack sets of every piece.
//!
//! Pawns, knights and kings always attack the same squares from a given
//! square, so their attacks are computed for all 64 squares at compile time.
//! Bishops, rooks and queens are stopped by the first piece on each ray;
//! their attacks are looked up in tables indexed with magic bitboards
//! ([`Magic`]), filled once when the engine starts (see [`init`]).

use std::sync::LazyLock;

use crate::board::{Bitboard, Board, Color, PieceKind, Square};

const KNIGHT_OFFSETS: [(i8, i8); 8] = [
    (-2, -1),
    (-2, 1),
    (-1, -2),
    (-1, 2),
    (1, -2),
    (1, 2),
    (2, -1),
    (2, 1),
];
const KING_OFFSETS: [(i8, i8); 8] = [
    (-1, -1),
    (-1, 0),
    (-1, 1),
    (0, -1),
    (0, 1),
    (1, -1),
    (1, 0),
    (1, 1),
];
const WHITE_PAWN_OFFSETS: [(i8, i8); 2] = [(-1, 1), (1, 1)];
const BLACK_PAWN_OFFSETS: [(i8, i8); 2] = [(-1, -1), (1, -1)];

const ROOK_DIRECTIONS: [(i8, i8); 4] = [(-1, 0), (1, 0), (0, -1), (0, 1)];
const BISHOP_DIRECTIONS: [(i8, i8); 4] = [(-1, -1), (-1, 1), (1, -1), (1, 1)];

static KNIGHT_ATTACKS: [u64; 64] = leaper_table(&KNIGHT_OFFSETS);
static KING_ATTACKS: [u64; 64] = leaper_table(&KING_OFFSETS);
static PAWN_ATTACKS: [[u64; 64]; 2] = [
    leaper_table(&WHITE_PAWN_OFFSETS),
    leaper_table(&BLACK_PAWN_OFFSETS),
];

static SLIDERS: LazyLock<SliderTables> = LazyLock::new(SliderTables::new);

/// Fills the sliding attack tables now, which takes a few milliseconds,
/// rather than during the first search, where the time counts against the
/// move.
pub fn init() {
    LazyLock::force(&SLIDERS);
}

/// The squares reached from every square by one jump of each offset.
const fn leaper_table(offsets: &[(i8, i8)]) -> [u64; 64] {
    let mut table = [0; 64];
    let mut square = 0;
    while square < 64 {
        let file = (square % 8) as i8;
        let rank = (square / 8) as i8;
        let mut i = 0;
        while i < offsets.len() {
            let (target_file, target_rank) = (file + offsets[i].0, rank + offsets[i].1);
            if target_file >= 0 && target_file < 8 && target_rank >= 0 && target_rank < 8 {
                table[square] |= 1u64 << (target_rank * 8 + target_file);
            }
            i += 1;
        }
        square += 1;
    }
    table
}

pub fn pawn_attacks(square: Square, color: Color) -> Bitboard {
    Bitboard(PAWN_ATTACKS[color.index()][square.index() as usize])
}

pub fn knight_attacks(square: Square) -> Bitboard {
    Bitboard(KNIGHT_ATTACKS[square.index() as usize])
}

pub fn king_attacks(square: Square) -> Bitboard {
    Bitboard(KING_ATTACKS[square.index() as usize])
}

pub fn bishop_attacks(square: Square, occupancy: Bitboard) -> Bitboard {
    let tables = &*SLIDERS;
    Bitboard(tables.attacks[tables.bishop[square.index() as usize].index(occupancy.0)])
}

pub fn rook_attacks(square: Square, occupancy: Bitboard) -> Bitboard {
    let tables = &*SLIDERS;
    Bitboard(tables.attacks[tables.rook[square.index() as usize].index(occupancy.0)])
}

pub fn queen_attacks(square: Square, occupancy: Bitboard) -> Bitboard {
    Bitboard(bishop_attacks(square, occupancy).0 | rook_attacks(square, occupancy).0)
}

pub fn piece_attacks(
    piece: PieceKind,
    square: Square,
    color: Color,
    occupancy: Bitboard,
) -> Bitboard {
    match piece {
        PieceKind::Pawn => pawn_attacks(square, color),
        PieceKind::Knight => knight_attacks(square),
        PieceKind::Bishop => bishop_attacks(square, occupancy),
        PieceKind::Rook => rook_attacks(square, occupancy),
        PieceKind::Queen => queen_attacks(square, occupancy),
        PieceKind::King => king_attacks(square),
    }
}

pub fn is_square_attacked(board: &Board, target: Square, by_color: Color) -> bool {
    is_square_attacked_through(board, target, by_color, board.occupancy())
}

/// Like [`is_square_attacked`], but sliding pieces are blocked by `occupancy`
/// instead of the board's pieces. A king testing the square it moves to
/// leaves itself out, so it cannot hide behind its own square from a slider.
///
/// Attacks are symmetric: a knight on `s` attacks `target` exactly when a
/// knight on `target` would attack `s`. So instead of generating the attacks
/// of every enemy piece, this generates each kind of attack once from
/// `target` and checks whether it hits an enemy piece of that kind. Pawns are
/// the exception to symmetry: they attack forward, so the attackers of a
/// square are found with the pawn attacks of the opposite color.
pub fn is_square_attacked_through(
    board: &Board,
    target: Square,
    by_color: Color,
    occupancy: Bitboard,
) -> bool {
    let attackers = board.color_bb(by_color).0;
    let pieces = |piece: PieceKind| board.piece_bb(piece).0 & attackers;
    let queens = pieces(PieceKind::Queen);

    pawn_attacks(target, by_color.opposite()).0 & pieces(PieceKind::Pawn) != 0
        || knight_attacks(target).0 & pieces(PieceKind::Knight) != 0
        || king_attacks(target).0 & pieces(PieceKind::King) != 0
        || bishop_attacks(target, occupancy).0 & (pieces(PieceKind::Bishop) | queens) != 0
        || rook_attacks(target, occupancy).0 & (pieces(PieceKind::Rook) | queens) != 0
}

/// Attacks of a sliding piece, found by walking each ray square by square
/// until the edge or the first occupied square, which is included. Slow but
/// plainly correct; the magic tables are filled from it.
fn ray_attacks(square: usize, occupancy: u64, directions: &[(i8, i8); 4]) -> u64 {
    let mut attacks = 0;
    for &(file_delta, rank_delta) in directions {
        let mut file = (square % 8) as i8 + file_delta;
        let mut rank = (square / 8) as i8 + rank_delta;
        while (0..8).contains(&file) && (0..8).contains(&rank) {
            let target = 1u64 << (rank * 8 + file);
            attacks |= target;
            if occupancy & target != 0 {
                break;
            }
            file += file_delta;
            rank += rank_delta;
        }
    }
    attacks
}

/// The squares whose occupancy can change a slider's attacks: its rays
/// without the last square before the edge, because a piece standing there
/// has nothing behind it to block.
fn relevant_mask(square: usize, directions: &[(i8, i8); 4]) -> u64 {
    let mut mask = 0;
    for &(file_delta, rank_delta) in directions {
        let mut file = (square % 8) as i8 + file_delta;
        let mut rank = (square / 8) as i8 + rank_delta;
        while (0..8).contains(&(file + file_delta)) && (0..8).contains(&(rank + rank_delta)) {
            mask |= 1u64 << (rank * 8 + file);
            file += file_delta;
            rank += rank_delta;
        }
    }
    mask
}

/// A magic bitboard: a perfect hash from the blockers on a slider's rays to
/// its attacks.
///
/// Only the squares of `mask` can block, so a rook has at most 2^12 different
/// blocker sets and a bishop 2^9. Multiplying the masked occupancy by `magic`
/// gathers its bits in the top `64 - shift` bits of the product, which then
/// index the slider's slice of the attack table. A magic is a number for which
/// no two blocker sets with different attacks land on the same index. Such
/// numbers are found by trying random ones; `ROOK_MAGICS` and `BISHOP_MAGICS`
/// were found by the `find_magics` test and are fixed, so the engine does not
/// search for them on every start.
#[derive(Clone, Copy, Debug, Default)]
struct Magic {
    mask: u64,
    magic: u64,
    shift: u32,
    offset: usize,
}

impl Magic {
    fn index(&self, occupancy: u64) -> usize {
        self.offset + ((occupancy & self.mask).wrapping_mul(self.magic) >> self.shift) as usize
    }
}

struct SliderTables {
    rook: [Magic; 64],
    bishop: [Magic; 64],
    /// The attack slices of all squares, rooks then bishops, about 840 kB.
    attacks: Vec<u64>,
}

impl SliderTables {
    fn new() -> Self {
        let mut attacks = Vec::new();
        let rook = std::array::from_fn(|square| {
            fill_slice(square, &ROOK_DIRECTIONS, ROOK_MAGICS[square], &mut attacks)
        });
        let bishop = std::array::from_fn(|square| {
            fill_slice(
                square,
                &BISHOP_DIRECTIONS,
                BISHOP_MAGICS[square],
                &mut attacks,
            )
        });
        Self {
            rook,
            bishop,
            attacks,
        }
    }
}

/// Appends the attack slice of a slider on `square` to `attacks`: its attacks
/// for every blocker set, each stored at the index the magic gives.
fn fill_slice(
    square: usize,
    directions: &[(i8, i8); 4],
    magic: u64,
    attacks: &mut Vec<u64>,
) -> Magic {
    let mask = relevant_mask(square, directions);
    let entry = Magic {
        mask,
        magic,
        shift: 64 - mask.count_ones(),
        offset: attacks.len(),
    };
    attacks.resize(attacks.len() + (1 << mask.count_ones()), 0);
    for blockers in subsets(mask) {
        attacks[entry.index(blockers)] = ray_attacks(square, blockers, directions);
    }
    entry
}

/// Every subset of `mask`, starting with the empty set, enumerated with the
/// carry-rippler trick: subtracting the mask and masking again counts
/// through its bits.
fn subsets(mask: u64) -> impl Iterator<Item = u64> {
    let mut next = Some(0u64);
    std::iter::from_fn(move || {
        let subset = next?;
        let following = subset.wrapping_sub(mask) & mask;
        next = (following != 0).then_some(following);
        Some(subset)
    })
}

#[rustfmt::skip]
const ROOK_MAGICS: [u64; 64] = [
    0x008000908064c000,
    0x0040200040001000,
    0x0180100080a0010a,
    0x8880041000800800,
    0x1200100201200804,
    0x0200020004011008,
    0x2180010000800600,
    0x0200005088210204,
    0x0400800040008021,
    0x0400400020005000,
    0x8240801000200080,
    0x8611001004200900,
    0x008180800c001800,
    0x0100800200800400,
    0x0a02000102000408,
    0x8020802300104280,
    0x0080004000402000,
    0xe010104000402000,
    0x0800808010002000,
    0xa280210008100100,
    0x0001818014000800,
    0xa002010100080400,
    0x0080240001020870,
    0x0001020004048845,
    0x0081826280004004,
    0x2020810900284000,
    0x0200100080802000,
    0x0200080080100080,
    0x8083080100100500,
    0x4406000901000400,
    0x0005020080800100,
    0x0090204200008114,
    0x0010400094800420,
    0x0900804000802002,
    0x0201001841002000,
    0x4100080080801000,
    0x4540040080800800,
    0x0002001004040020,
    0x0281195814001002,
    0x1240800040800100,
    0x0880042000524004,
    0x02c080410206002c,
    0x0801200241050010,
    0x8400080010008080,
    0x0008000500090010,
    0x0082009084020008,
    0x4012000108020004,
    0x9000104d08860004,
    0x2004204114800100,
    0x0148802112400300,
    0x0202842000100880,
    0x001b080080900080,
    0x001a002008100600,
    0x0004008004020080,
    0x5181000600040300,
    0x0000044401128a00,
    0x8044110480002441,
    0x2008110084402202,
    0x90806005090010c1,
    0x000420310a004a42,
    0x0023001004020801,
    0x0882001008040102,
    0x000230088118020c,
    0x0000019025040042,
];
#[rustfmt::skip]
const BISHOP_MAGICS: [u64; 64] = [
    0x0045010808008680,
    0x2002080204004898,
    0x0210009a10400006,
    0x0824050200810200,
    0x0006061105004090,
    0x00010108c0000000,
    0x0814040282104004,
    0x0012012201106800,
    0x10823014100c1040,
    0x0080c2088802808c,
    0x0281108410404000,
    0x0101212041826200,
    0x0020141028221058,
    0x2201020202200202,
    0x000082a801482000,
    0x0000008401411044,
    0x0007103014300404,
    0x0002091110010100,
    0x42140012040c0808,
    0x0800808802004020,
    0x90c4004210140000,
    0x0800200900a01000,
    0x00d0400201108810,
    0x80820183814412a0,
    0x00a01008202202b4,
    0x01c2021a09500402,
    0x0084440208042400,
    0x800400400c090100,
    0xba10040010802100,
    0xd182009006005000,
    0x5011021001009004,
    0x0020420200510400,
    0x0292104000468800,
    0x00043009091c0500,
    0x0280441000020025,
    0x0042820080080080,
    0x0440101010010040,
    0x1000900100808080,
    0x0108108120089800,
    0x0044010200012682,
    0xc002500420900400,
    0x0040482210710800,
    0x0002060024000200,
    0x0281020a44000800,
    0xa0021200a4000200,
    0x0001301000840840,
    0x2868500108444220,
    0x0004111041000200,
    0x8044020842080200,
    0x0000220104210200,
    0x0000021201044000,
    0x0000280884040028,
    0x4012114010858003,
    0x0000081004082b88,
    0x3892700508208002,
    0x00220a041b060400,
    0x0812020284014881,
    0x010434a282103100,
    0x0490400824020800,
    0x4a20002c00208800,
    0x000000a011020200,
    0x4002940a02482202,
    0x5100100202140406,
    0x02102000840540c1,
];

#[cfg(test)]
mod tests {
    use super::*;

    /// xorshift64* pseudo-random numbers.
    struct Random(u64);

    impl Random {
        fn next(&mut self) -> u64 {
            self.0 ^= self.0 >> 12;
            self.0 ^= self.0 << 25;
            self.0 ^= self.0 >> 27;
            self.0.wrapping_mul(0x2545_f491_4f6c_dd1d)
        }
    }

    /// Searches for a magic for a slider on `square` by trying random numbers
    /// until one maps every blocker set to an index without a conflict.
    fn find_magic(square: usize, directions: &[(i8, i8); 4], random: &mut Random) -> u64 {
        let mask = relevant_mask(square, directions);
        let shift = 64 - mask.count_ones();
        let blockers: Vec<(u64, u64)> = subsets(mask)
            .map(|subset| (subset, ray_attacks(square, subset, directions)))
            .collect();
        let mut table = vec![0; blockers.len()];
        // `filled[i] == attempt` marks entries written by the current
        // attempt, so the table is not cleared between attempts.
        let mut filled = vec![0u32; blockers.len()];
        let mut attempt = 0;
        loop {
            // Numbers with few set bits make good magics.
            let magic = random.next() & random.next() & random.next();
            // A magic must move enough mask bits into the index bits.
            if (mask.wrapping_mul(magic) >> 56).count_ones() < 6 {
                continue;
            }
            attempt += 1;
            let collision_free = blockers.iter().all(|&(occupancy, attacks)| {
                let index = (occupancy.wrapping_mul(magic) >> shift) as usize;
                if filled[index] != attempt {
                    filled[index] = attempt;
                    table[index] = attacks;
                    true
                } else {
                    // Blocker sets may share an index if their attacks agree.
                    table[index] == attacks
                }
            });
            if collision_free {
                return magic;
            }
        }
    }

    /// Prints a new set of magics for `ROOK_MAGICS` and `BISHOP_MAGICS`:
    /// `cargo test --release find_magics -- --ignored --nocapture`.
    #[test]
    #[ignore = "generates the magic constants; run with --ignored --nocapture"]
    fn find_magics() {
        let mut random = Random(0x2545_f491_4f6c_dd1d);
        for (name, directions) in [("ROOK", &ROOK_DIRECTIONS), ("BISHOP", &BISHOP_DIRECTIONS)] {
            println!("const {name}_MAGICS: [u64; 64] = [");
            for square in 0..64 {
                println!(
                    "    0x{:016x},",
                    find_magic(square, directions, &mut random)
                );
            }
            println!("];");
        }
    }

    #[test]
    fn magics_give_the_attacks_of_every_blocker_set() {
        for square in Square::ALL {
            let index = square.index() as usize;
            for (directions, lookup) in [
                (
                    &ROOK_DIRECTIONS,
                    rook_attacks as fn(Square, Bitboard) -> Bitboard,
                ),
                (&BISHOP_DIRECTIONS, bishop_attacks),
            ] {
                for blockers in subsets(relevant_mask(index, directions)) {
                    assert_eq!(
                        lookup(square, Bitboard(blockers)).0,
                        ray_attacks(index, blockers, directions),
                        "{square:?} with blockers {blockers:#x}"
                    );
                }
            }
        }
    }

    #[test]
    fn knight_has_eight_attacks_in_center() {
        assert_eq!(knight_attacks(Square::D4).count_bits(), 8);
    }

    #[test]
    fn king_has_three_attacks_in_corner() {
        assert_eq!(king_attacks(Square::A1).count_bits(), 3);
    }

    #[test]
    fn knight_in_corner_does_not_wrap_around_the_board() {
        let attacks = knight_attacks(Square::H1);
        assert_eq!(attacks.count_bits(), 2);
        assert!(attacks.test_bit(Square::F2));
        assert!(attacks.test_bit(Square::G3));
    }

    #[test]
    fn pawn_attacks_depend_on_color() {
        assert!(pawn_attacks(Square::E4, Color::White).test_bit(Square::D5));
        assert!(pawn_attacks(Square::E4, Color::White).test_bit(Square::F5));
        assert!(pawn_attacks(Square::E4, Color::Black).test_bit(Square::D3));
        assert!(pawn_attacks(Square::E4, Color::Black).test_bit(Square::F3));
        assert_eq!(pawn_attacks(Square::A4, Color::White).count_bits(), 1);
    }

    #[test]
    fn rook_attacks_stop_at_blocker() {
        let mut occupancy = Bitboard::empty();
        occupancy.set_bit(Square::D6);
        occupancy.set_bit(Square::F4);
        let attacks = rook_attacks(Square::D4, occupancy);

        assert!(attacks.test_bit(Square::D6));
        assert!(!attacks.test_bit(Square::D7));
        assert!(attacks.test_bit(Square::F4));
        assert!(!attacks.test_bit(Square::G4));
    }

    #[test]
    fn bishop_attacks_are_blocked_on_each_ray() {
        let mut occupancy = Bitboard::empty();
        occupancy.set_bit(Square::F6);
        let attacks = bishop_attacks(Square::D4, occupancy);

        assert!(attacks.test_bit(Square::E5));
        assert!(attacks.test_bit(Square::F6));
        assert!(!attacks.test_bit(Square::G7));
        assert!(attacks.test_bit(Square::C5));
    }

    #[test]
    fn magic_lookups_match_ray_walking() {
        let mut random = Random(42);
        for square in Square::ALL {
            for _ in 0..1000 {
                // Sparse and dense boards both occur in games.
                let occupancy = if random.next() & 1 == 0 {
                    random.next() & random.next()
                } else {
                    random.next() | random.next()
                };
                let index = square.index() as usize;
                assert_eq!(
                    rook_attacks(square, Bitboard(occupancy)).0,
                    ray_attacks(index, occupancy, &ROOK_DIRECTIONS),
                    "rook on {square:?}"
                );
                assert_eq!(
                    bishop_attacks(square, Bitboard(occupancy)).0,
                    ray_attacks(index, occupancy, &BISHOP_DIRECTIONS),
                    "bishop on {square:?}"
                );
            }
        }
    }

    #[test]
    fn relevant_masks_have_the_expected_size() {
        assert_eq!(
            relevant_mask(Square::A1.index() as usize, &ROOK_DIRECTIONS).count_ones(),
            12
        );
        assert_eq!(
            relevant_mask(Square::D4.index() as usize, &ROOK_DIRECTIONS).count_ones(),
            10
        );
        assert_eq!(
            relevant_mask(Square::A1.index() as usize, &BISHOP_DIRECTIONS).count_ones(),
            6
        );
        assert_eq!(
            relevant_mask(Square::D4.index() as usize, &BISHOP_DIRECTIONS).count_ones(),
            9
        );
    }

    #[test]
    fn attacked_square_is_detected_from_board() {
        let mut board = Board::empty();
        board.set_piece(Square::E4, PieceKind::Rook, Color::White);
        assert!(is_square_attacked(&board, Square::E8, Color::White));
        assert!(!is_square_attacked(&board, Square::D8, Color::White));
    }

    #[test]
    fn pawns_attack_forward_only() {
        let mut board = Board::empty();
        board.set_piece(Square::E4, PieceKind::Pawn, Color::White);
        assert!(is_square_attacked(&board, Square::D5, Color::White));
        assert!(!is_square_attacked(&board, Square::D3, Color::White));
    }
}
