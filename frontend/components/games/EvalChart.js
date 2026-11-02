import { evalPawns, MATE_PAWNS, scoreForWhite } from '../../lib/chess'

const WIDTH = 600
const HEIGHT = 140
const PADDING = 6
const LIMIT = 5

// Evaluation after every move from White's point of view, clipped to ±LIMIT
// pawns: what the engine that moved reported, and Stockfish's analysis when
// the game was analyzed. Clicking the chart jumps to the nearest move.
export default function EvalChart({ moves, startsWithBlack, ply, onSelect }) {
  if (moves.length === 0) {
    return null
  }
  const step = (WIDTH - 2 * PADDING) / Math.max(1, moves.length - 1)
  const x = (index) => PADDING + index * step
  const y = (pawns) =>
    HEIGHT / 2 -
    (Math.max(-LIMIT, Math.min(LIMIT, pawns)) / LIMIT) * (HEIGHT / 2 - PADDING)

  const series = (score) =>
    moves
      .map((move, index) => {
        const value = score(move, index)
        return value === null ? null : { index, value }
      })
      .filter(Boolean)
  const toLine = (points) =>
    points
      .map(
        ({ index, value }) => `${x(index).toFixed(1)},${y(value).toFixed(1)}`
      )
      .join(' ')

  const engine = series((move, index) =>
    scoreForWhite(move.info, (index % 2 === 0) !== startsWithBlack)
  )
  const stockfish = series((move) => evalPawns(move.analysis?.eval_after))
  // White's share of the board fills from the bottom up to the line.
  const filled = stockfish.length > 0 ? stockfish : engine
  const area =
    filled.length > 0
      ? `${x(filled[0].index)},${HEIGHT} ${toLine(filled)} ${x(
          filled[filled.length - 1].index
        )},${HEIGHT}`
      : ''

  function select(event) {
    const box = event.currentTarget.getBoundingClientRect()
    const position = ((event.clientX - box.left) / box.width) * WIDTH
    const index = Math.round((position - PADDING) / step)
    onSelect(Math.max(0, Math.min(moves.length - 1, index)) + 1)
  }

  return (
    <div>
      <div className="mb-1 flex flex-wrap justify-between gap-2 text-xs text-gray-500">
        <span className="flex items-center gap-3">
          Evaluation (White)
          <span className="flex items-center gap-1">
            <span className="inline-block h-0.5 w-4 bg-sky-500" /> engine
          </span>
          {stockfish.length > 0 && (
            <span className="flex items-center gap-1">
              <span className="inline-block h-0.5 w-4 bg-violet-500" />{' '}
              Stockfish
            </span>
          )}
        </span>
        <span>
          ±{LIMIT} pawns · mate shown as ±{MATE_PAWNS}
        </span>
      </div>
      <svg
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        className="h-32 w-full cursor-pointer rounded border border-gray-200 bg-gray-800"
        preserveAspectRatio="none"
        onClick={select}
        role="img"
        aria-label="Evaluation chart; click to jump to a move"
      >
        <polygon points={area} className="fill-gray-100" />
        <line
          x1={0}
          x2={WIDTH}
          y1={HEIGHT / 2}
          y2={HEIGHT / 2}
          className="stroke-gray-400"
          strokeWidth={1}
        />
        <polyline
          points={toLine(engine)}
          fill="none"
          className="stroke-sky-500"
          strokeWidth={stockfish.length > 0 ? 1.5 : 2}
          vectorEffect="non-scaling-stroke"
        />
        {stockfish.length > 0 && (
          <polyline
            points={toLine(stockfish)}
            fill="none"
            className="stroke-violet-500"
            strokeWidth={2}
            vectorEffect="non-scaling-stroke"
          />
        )}
        {ply > 0 && (
          <line
            x1={x(ply - 1)}
            x2={x(ply - 1)}
            y1={0}
            y2={HEIGHT}
            className="stroke-amber-500"
            strokeWidth={2}
            vectorEffect="non-scaling-stroke"
          />
        )}
      </svg>
    </div>
  )
}
