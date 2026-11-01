import { formatNumber, formatScore, sanMoves } from '../lib/chess'

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

// Search telemetry of one engine move.
//
// move: { color, san, fenBefore, info, thinkTimeMs }, where info is what the
// engine reported in its last UCI "info" line before "bestmove".
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

  return (
    <section className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <h2 className="mb-3 text-sm font-semibold">{title}</h2>
      {body}
    </section>
  )
}
