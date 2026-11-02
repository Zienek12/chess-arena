import {
  CLASSIFICATIONS,
  formatEval,
  formatNumber,
  formatScore,
  sanMoves,
} from '../lib/chess'

function hasTelemetry(info) {
  return Boolean(
    info &&
      (info.depth ||
        info.nodes ||
        info.score_cp !== undefined ||
        info.score_mate !== undefined)
  )
}

function Stat({ label, value, hint }) {
  return (
    <div title={hint}>
      <dt className="text-xs uppercase tracking-wide text-gray-500">{label}</dt>
      <dd className="font-mono text-lg font-semibold">{value}</dd>
    </div>
  )
}

// Stockfish's verdict on a played move.
function AnalysisSection({ move }) {
  const { analysis } = move
  const classification = CLASSIFICATIONS[analysis.classification]
  const [best] = analysis.best_move
    ? sanMoves(move.fenBefore, [analysis.best_move])
    : ['']
  const line = analysis.best_pv?.length
    ? sanMoves(move.fenBefore, analysis.best_pv).join(' ')
    : ''
  return (
    <div className="mt-4 border-t border-gray-100 pt-3">
      <p className="mb-3 flex items-center gap-2 text-sm text-gray-600">
        Stockfish:
        <span className={`font-semibold ${classification.className}`}>
          {classification.symbol} {classification.label}
        </span>
      </p>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3">
        <Stat
          label="Best move"
          value={best || '–'}
          hint={`Stockfish's choice at depth ${analysis.depth}`}
        />
        <Stat
          label="Eval"
          value={`${formatEval(analysis.eval_before)} → ${formatEval(
            analysis.eval_after
          )}`}
          hint="Stockfish's evaluation before and after the move, from White's point of view"
        />
        <Stat
          label="Loss"
          value={`${analysis.cp_loss} cp`}
          hint="Centipawns lost compared to the best move (capped at 1000)"
        />
        <Stat
          label="Accuracy"
          value={`${analysis.accuracy.toFixed(0)}%`}
          hint="Accuracy of the move from the drop in winning chances (Lichess formula)"
        />
      </dl>
      {line && analysis.classification !== 'best' && (
        <div className="mt-3">
          <p className="text-xs uppercase tracking-wide text-gray-500">
            Stockfish line
          </p>
          <p className="font-mono text-sm">{line}</p>
        </div>
      )}
    </div>
  )
}

// Search telemetry of one engine move, and Stockfish's analysis when present.
//
// move: { color, san, fenBefore, info, thinkTimeMs, analysis }, where info is
// what the engine reported in its last UCI "info" line before "bestmove".
export default function TelemetryPanel({
  move,
  thinking = false,
  title = 'Engine search',
}) {
  let body
  if (thinking) {
    body = <p className="text-sm text-gray-500">Engine is thinking...</p>
  } else if (!move) {
    body = <p className="text-sm text-gray-500">No engine move yet.</p>
  } else if (!hasTelemetry(move.info)) {
    body = (
      <p className="text-sm text-gray-500">
        {move.san} — the engine did not report search telemetry.
      </p>
    )
  } else {
    const { info } = move
    const depth =
      info.seldepth && info.seldepth !== info.depth
        ? `${info.depth ?? 0} / ${info.seldepth}`
        : String(info.depth ?? 0)
    const pv = info.pv?.length
      ? sanMoves(move.fenBefore, info.pv).join(' ')
      : ''
    body = (
      <>
        <p className="mb-3 text-sm text-gray-600">
          Move{' '}
          <span className="font-mono font-semibold text-gray-900">
            {move.san}
          </span>
        </p>
        <dl className="grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3">
          <Stat
            label="Depth"
            value={depth}
            hint="Completed search depth in plies / deepest ply reached"
          />
          <Stat
            label="Eval"
            value={formatScore(info, move.color === 'w') || '–'}
            hint="Engine evaluation from White's point of view, in pawns; #n is mate in n"
          />
          <Stat
            label="Nodes"
            value={(info.nodes ?? 0).toLocaleString()}
            hint="Positions the engine visited"
          />
          <Stat
            label="Speed"
            value={info.nps ? `${formatNumber(info.nps)}/s` : '–'}
            hint="Nodes per second"
          />
          <Stat
            label="Engine time"
            value={`${info.time_ms ?? 0} ms`}
            hint="Search time reported by the engine"
          />
          <Stat
            label="Think time"
            value={
              move.thinkTimeMs !== undefined ? `${move.thinkTimeMs} ms` : '–'
            }
            hint="Time from go to bestmove measured by the arena, including communication"
          />
        </dl>
        {pv && (
          <div className="mt-3">
            <p className="text-xs uppercase tracking-wide text-gray-500">
              Principal variation
            </p>
            <p className="font-mono text-sm">{pv}</p>
          </div>
        )}
      </>
    )
  }
  if (!thinking && move?.analysis) {
    body = (
      <>
        {body}
        <AnalysisSection move={move} />
      </>
    )
  }

  return (
    <section className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <h2 className="mb-3 text-sm font-semibold">{title}</h2>
      {body}
    </section>
  )
}
