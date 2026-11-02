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

// Score in pawns as seen from White, or null without a score. Mates count as
// MATE_PAWNS so they stay on a chart.
export const MATE_PAWNS = 10

export function scoreForWhite(info, moverIsWhite) {
  const sign = moverIsWhite ? 1 : -1
  if (info?.score_mate !== undefined && info?.score_mate !== null) {
    const mate = info.score_mate * sign
    // "mate 0" means the mover is checkmated.
    return (mate > 0 ? 1 : mate < 0 ? -1 : -sign) * MATE_PAWNS
  }
  if (info?.score_cp !== undefined && info?.score_cp !== null) {
    return (info.score_cp * sign) / 100
  }
  return null
}

// Summarizes the searches of each engine in the same shape as the
// tournament telemetry returned by the API.
export function summarizeTelemetry(moves, engineIds) {
  return engineIds.map((engineId) => {
    const own = moves.filter((move) => move.engine_id === engineId)
    const withDepth = own.filter((move) => move.info?.depth)
    const average = (values) =>
      values.length === 0
        ? 0
        : values.reduce((sum, value) => sum + value, 0) / values.length
    const reported = (key) =>
      own.map((move) => move.info?.[key]).filter((value) => value)
    return {
      engine_id: engineId,
      moves: own.length,
      moves_with_info: withDepth.length,
      average_depth: average(withDepth.map((move) => move.info.depth)),
      max_depth: Math.max(0, ...withDepth.map((move) => move.info.depth)),
      average_nodes: average(reported('nodes')),
      total_nodes: reported('nodes').reduce((sum, value) => sum + value, 0),
      average_nps: average(reported('nps')),
      average_think_time_ms: average(
        own
          .map((move) => move.think_time_ms)
          .filter((value) => value !== undefined && value !== null)
      ),
      ...summarizeQuality(own.map((move) => move.analysis).filter(Boolean)),
    }
  })
}

// Move quality in the shape of the API's move_quality fields.
function summarizeQuality(analyses) {
  const count = (classification) =>
    analyses.filter((analysis) => analysis.classification === classification)
      .length
  const average = (key) =>
    analyses.length === 0
      ? 0
      : analyses.reduce((sum, analysis) => sum + analysis[key], 0) /
        analyses.length
  return {
    analyzed_moves: analyses.length,
    average_cp_loss: average('cp_loss'),
    accuracy: average('accuracy'),
    best_moves: count('best'),
    inaccuracies: count('inaccuracy'),
    mistakes: count('mistake'),
    blunders: count('blunder'),
  }
}

// Formats an analysis evaluation, which is already from White's view.
export function formatEval(evaluation) {
  if (!evaluation) {
    return ''
  }
  if (evaluation.mate !== undefined && evaluation.mate !== null) {
    if (evaluation.mate === 0) {
      return '#'
    }
    return `${evaluation.mate > 0 ? '' : '-'}#${Math.abs(evaluation.mate)}`
  }
  const pawns = evaluation.cp / 100
  return `${pawns > 0 ? '+' : ''}${pawns.toFixed(2)}`
}

// Analysis evaluation in pawns from White's view, with mates as MATE_PAWNS.
export function evalPawns(evaluation) {
  if (!evaluation) {
    return null
  }
  if (evaluation.mate !== undefined && evaluation.mate !== null) {
    return Math.sign(evaluation.cp) * MATE_PAWNS
  }
  return evaluation.cp / 100
}

// Annotation symbol and colors for a move classification.
export const CLASSIFICATIONS = {
  best: { symbol: '★', label: 'Best move', className: 'text-emerald-600' },
  good: { symbol: '', label: 'Good move', className: 'text-gray-500' },
  inaccuracy: {
    symbol: '?!',
    label: 'Inaccuracy',
    className: 'text-amber-600',
  },
  mistake: { symbol: '?', label: 'Mistake', className: 'text-orange-600' },
  blunder: { symbol: '??', label: 'Blunder', className: 'text-rose-600' },
}
