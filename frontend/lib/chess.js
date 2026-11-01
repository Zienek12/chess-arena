import { Chess } from 'chess.js'

const SELECTED_STYLE = { background: 'rgba(255, 213, 79, 0.6)' }
const LAST_MOVE_STYLE = { background: 'rgba(255, 235, 59, 0.35)' }
const TARGET_STYLE = {
  background:
    'radial-gradient(circle, rgba(0, 0, 0, 0.22) 22%, transparent 24%)',
  cursor: 'pointer',
}
const CAPTURE_STYLE = {
  background:
    'radial-gradient(circle, transparent 56%, rgba(0, 0, 0, 0.22) 58%, rgba(0, 0, 0, 0.22) 70%, transparent 72%)',
  cursor: 'pointer',
}

export function parseUci(uci) {
  return {
    from: uci.slice(0, 2),
    to: uci.slice(2, 4),
    promotion: uci[4] || undefined,
  }
}

// Returns the chess.js move, or null when the move is not legal.
export function applyUciMove(game, uci) {
  try {
    return game.move(parseUci(uci))
  } catch {
    return null
  }
}

// Legal destination squares of the piece on square.
export function legalTargets(game, square) {
  if (!square) {
    return []
  }
  return game.moves({ square, verbose: true })
}

// Square styles for the last move, the selected piece and where it can go.
export function boardHighlights({ lastMove, selected, targets = [] }) {
  const styles = {}
  if (lastMove) {
    styles[lastMove.from] = LAST_MOVE_STYLE
    styles[lastMove.to] = LAST_MOVE_STYLE
  }
  if (selected) {
    styles[selected] = SELECTED_STYLE
  }
  for (const move of targets) {
    styles[move.to] = {
      ...styles[move.to],
      ...(move.captured ? CAPTURE_STYLE : TARGET_STYLE),
    }
  }
  return styles
}

// Replays UCI moves from startFen and returns SAN notation for each one.
export function sanMoves(startFen, uciMoves) {
  const game = new Chess(startFen)
  return uciMoves.map((uci) => applyUciMove(game, uci)?.san || uci)
}

// Formats an engine score as seen from White. Engines report the score from
// the side to move, which is the side that played the move.
export function formatScore(info, moverIsWhite) {
  const sign = moverIsWhite ? 1 : -1
  if (info?.score_mate !== undefined && info?.score_mate !== null) {
    const mate = info.score_mate * sign
    return mate === 0 ? '#' : `${mate > 0 ? '' : '-'}#${Math.abs(mate)}`
  }
  if (info?.score_cp !== undefined && info?.score_cp !== null) {
    const pawns = (info.score_cp * sign) / 100
    return `${pawns > 0 ? '+' : ''}${pawns.toFixed(2)}`
  }
  return ''
}

export function formatNumber(value) {
  if (!value) {
    return ''
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(1)}M`
  }
  if (value >= 1_000) {
    return `${(value / 1_000).toFixed(1)}k`
  }
  return String(value)
}
