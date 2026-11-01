// Balanced opening lines for engine tournaments. Each line ends after a few
// moves of mainline theory, so engines start from varied but equal positions.
// With alternating colors every position is played once with each color.
export const OPENING_SUITE_NAME = 'Balanced openings (50)'

export const OPENINGS = [
  // 1. e4 e5
  [
    'Ruy Lopez, Morphy Defence',
    '1. e4 e5 2. Nf3 Nc6 3. Bb5 a6 4. Ba4 Nf6 5. O-O Be7',
  ],
  [
    'Ruy Lopez, Berlin Defence',
    '1. e4 e5 2. Nf3 Nc6 3. Bb5 Nf6 4. O-O Nxe4 5. d4 Nd6',
  ],
  [
    'Ruy Lopez, Exchange Variation',
    '1. e4 e5 2. Nf3 Nc6 3. Bb5 a6 4. Bxc6 dxc6 5. O-O f6',
  ],
  [
    'Italian Game, Giuoco Pianissimo',
    '1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 4. c3 Nf6 5. d3 d6',
  ],
  [
    'Two Knights Defence',
    '1. e4 e5 2. Nf3 Nc6 3. Bc4 Nf6 4. d3 Be7 5. O-O O-O',
  ],
  ['Scotch Game', '1. e4 e5 2. Nf3 Nc6 3. d4 exd4 4. Nxd4 Nf6 5. Nxc6 bxc6'],
  ['Four Knights Game', '1. e4 e5 2. Nf3 Nc6 3. Nc3 Nf6 4. Bb5 Bb4 5. O-O O-O'],
  ['Petrov Defence', '1. e4 e5 2. Nf3 Nf6 3. Nxe5 d6 4. Nf3 Nxe4 5. d4 d5'],
  ['Vienna Game', '1. e4 e5 2. Nc3 Nf6 3. g3 d5 4. exd5 Nxd5 5. Bg2 Nxc3'],
  [
    "King's Gambit Declined",
    '1. e4 e5 2. f4 Bc5 3. Nf3 d6 4. c3 Nf6 5. d4 exd4',
  ],
  ['Philidor Defence', '1. e4 e5 2. Nf3 d6 3. d4 Nf6 4. Nc3 Nbd7 5. Bc4 Be7'],
  // Sicilian
  ['Sicilian, Najdorf', '1. e4 c5 2. Nf3 d6 3. d4 cxd4 4. Nxd4 Nf6 5. Nc3 a6'],
  ['Sicilian, Dragon', '1. e4 c5 2. Nf3 d6 3. d4 cxd4 4. Nxd4 Nf6 5. Nc3 g6'],
  [
    'Sicilian, Classical',
    '1. e4 c5 2. Nf3 d6 3. d4 cxd4 4. Nxd4 Nf6 5. Nc3 Nc6',
  ],
  [
    'Sicilian, Scheveningen',
    '1. e4 c5 2. Nf3 e6 3. d4 cxd4 4. Nxd4 Nf6 5. Nc3 d6',
  ],
  [
    'Sicilian, Taimanov',
    '1. e4 c5 2. Nf3 e6 3. d4 cxd4 4. Nxd4 Nc6 5. Nc3 Qc7',
  ],
  [
    'Sicilian, Sveshnikov',
    '1. e4 c5 2. Nf3 Nc6 3. d4 cxd4 4. Nxd4 Nf6 5. Nc3 e5',
  ],
  ['Sicilian, Rossolimo', '1. e4 c5 2. Nf3 Nc6 3. Bb5 g6 4. O-O Bg7 5. c3 Nf6'],
  ['Sicilian, Alapin', '1. e4 c5 2. c3 Nf6 3. e5 Nd5 4. d4 cxd4 5. Nf3 Nc6'],
  ['Sicilian, Closed', '1. e4 c5 2. Nc3 Nc6 3. g3 g6 4. Bg2 Bg7 5. d3 d6'],
  // Other replies to 1. e4
  [
    'French, Winawer',
    '1. e4 e6 2. d4 d5 3. Nc3 Bb4 4. e5 c5 5. a3 Bxc3+ 6. bxc3 Ne7',
  ],
  ['French, Classical', '1. e4 e6 2. d4 d5 3. Nc3 Nf6 4. Bg5 Be7 5. e5 Nfd7'],
  ['French, Tarrasch', '1. e4 e6 2. d4 d5 3. Nd2 Nf6 4. e5 Nfd7 5. Bd3 c5'],
  ['French, Advance', '1. e4 e6 2. d4 d5 3. e5 c5 4. c3 Nc6 5. Nf3 Qb6'],
  [
    'Caro-Kann, Classical',
    '1. e4 c6 2. d4 d5 3. Nc3 dxe4 4. Nxe4 Bf5 5. Ng3 Bg6',
  ],
  ['Caro-Kann, Advance', '1. e4 c6 2. d4 d5 3. e5 Bf5 4. Nf3 e6 5. Be2 c5'],
  ['Pirc Defence', '1. e4 d6 2. d4 Nf6 3. Nc3 g6 4. Nf3 Bg7 5. Be2 O-O'],
  [
    'Scandinavian Defence',
    '1. e4 d5 2. exd5 Qxd5 3. Nc3 Qa5 4. d4 Nf6 5. Nf3 c6',
  ],
  ['Alekhine Defence', '1. e4 Nf6 2. e5 Nd5 3. d4 d6 4. Nf3 Bg4 5. Be2 e6'],
  // 1. d4
  [
    "Queen's Gambit Declined",
    '1. d4 d5 2. c4 e6 3. Nc3 Nf6 4. Bg5 Be7 5. e3 O-O',
  ],
  [
    "Queen's Gambit Accepted",
    '1. d4 d5 2. c4 dxc4 3. Nf3 Nf6 4. e3 e6 5. Bxc4 c5',
  ],
  ['Slav Defence', '1. d4 d5 2. c4 c6 3. Nf3 Nf6 4. Nc3 dxc4 5. a4 Bf5'],
  ['Semi-Slav Defence', '1. d4 d5 2. c4 c6 3. Nf3 Nf6 4. Nc3 e6 5. e3 Nbd7'],
  [
    "Queen's Gambit, Tarrasch",
    '1. d4 d5 2. c4 e6 3. Nc3 c5 4. cxd5 exd5 5. Nf3 Nc6',
  ],
  [
    'Nimzo-Indian, Rubinstein',
    '1. d4 Nf6 2. c4 e6 3. Nc3 Bb4 4. e3 O-O 5. Bd3 d5',
  ],
  [
    'Nimzo-Indian, Classical',
    '1. d4 Nf6 2. c4 e6 3. Nc3 Bb4 4. Qc2 O-O 5. a3 Bxc3+ 6. Qxc3 b6',
  ],
  [
    "Queen's Indian Defence",
    '1. d4 Nf6 2. c4 e6 3. Nf3 b6 4. g3 Ba6 5. b3 Bb4+',
  ],
  ['Catalan Opening', '1. d4 Nf6 2. c4 e6 3. g3 d5 4. Bg2 Be7 5. Nf3 O-O'],
  [
    "King's Indian, Classical",
    '1. d4 Nf6 2. c4 g6 3. Nc3 Bg7 4. e4 d6 5. Nf3 O-O 6. Be2 e5',
  ],
  [
    'Grünfeld Defence',
    '1. d4 Nf6 2. c4 g6 3. Nc3 d5 4. cxd5 Nxd5 5. e4 Nxc3 6. bxc3 Bg7',
  ],
  ['Benoni Defence', '1. d4 Nf6 2. c4 c5 3. d5 e6 4. Nc3 exd5 5. cxd5 d6'],
  ['Dutch, Stonewall', '1. d4 f5 2. g3 Nf6 3. Bg2 e6 4. Nf3 d5 5. O-O Bd6'],
  ['London System', '1. d4 d5 2. Nf3 Nf6 3. Bf4 e6 4. e3 c5 5. c3 Nc6'],
  [
    'Bogo-Indian Defence',
    '1. d4 Nf6 2. c4 e6 3. Nf3 Bb4+ 4. Bd2 Qe7 5. g3 Nc6',
  ],
  // Flank openings
  [
    'English, Symmetrical',
    '1. c4 c5 2. Nc3 Nc6 3. g3 g6 4. Bg2 Bg7 5. Nf3 Nf6',
  ],
  [
    'English, Reversed Sicilian',
    '1. c4 e5 2. Nc3 Nf6 3. Nf3 Nc6 4. g3 d5 5. cxd5 Nxd5',
  ],
  [
    'English, Anglo-Indian',
    '1. c4 Nf6 2. Nc3 e6 3. Nf3 d5 4. d4 Be7 5. Bf4 O-O',
  ],
  ['Réti Opening', '1. Nf3 d5 2. g3 Nf6 3. Bg2 c6 4. O-O Bg4 5. d3 Nbd7'],
  [
    "King's Indian Attack",
    '1. Nf3 Nf6 2. g3 g6 3. Bg2 Bg7 4. O-O O-O 5. d3 d6',
  ],
  ["Bird's Opening", '1. f4 d5 2. Nf3 Nf6 3. e3 g6 4. Be2 Bg7 5. O-O O-O'],
]

// Every opening as a PGN game; the position after the moves is imported.
export function openingsPGN() {
  return OPENINGS.map(
    ([name, moves]) => `[Event "${name}"]\n\n${moves} *\n`
  ).join('\n')
}
