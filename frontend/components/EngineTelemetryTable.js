import { CLASSIFICATIONS, formatNumber } from '../lib/chess'

const cell = 'py-2 pr-3 text-right'

// Search telemetry per engine, and the quality of its moves once Stockfish
// analyzed them. Rows come from summarizeTelemetry or the API.
export default function EngineTelemetryTable({ rows, names }) {
  const analyzed = rows.some((row) => row.analyzed_moves > 0)
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="text-xs uppercase text-gray-500">
          <tr>
            <th className="py-2 pr-3 text-left">Engine</th>
            <th className={cell}>Moves</th>
            <th className={cell}>Avg depth</th>
            <th className={cell}>Max depth</th>
            <th className={cell}>Avg nodes</th>
            <th className={cell}>Total nodes</th>
            <th className={cell}>Avg NPS</th>
            <th className={cell}>Avg time</th>
            {analyzed && (
              <>
                <th
                  className={cell}
                  title="Average accuracy per move (Lichess formula)"
                >
                  Accuracy
                </th>
                <th className={cell} title="Average centipawn loss per move">
                  ACPL
                </th>
                <th
                  className="py-2 text-right"
                  title="Inaccuracies / mistakes / blunders"
                >
                  ?! / ? / ??
                </th>
              </>
            )}
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {rows.map((row, index) => (
            <tr key={`${row.engine_id}-${index}`}>
              <td className="py-2 pr-3 font-medium">
                {names[row.engine_id] || 'Engine'}
              </td>
              <td className={cell}>{row.moves}</td>
              {row.moves_with_info === 0 ? (
                <td colSpan={5} className={`${cell} text-xs text-gray-500`}>
                  {row.moves === 0 ? '' : 'No search telemetry reported'}
                </td>
              ) : (
                <>
                  <td className={cell}>{row.average_depth.toFixed(1)}</td>
                  <td className={cell}>{row.max_depth}</td>
                  <td className={cell}>
                    {formatNumber(Math.round(row.average_nodes))}
                  </td>
                  <td className={cell}>{formatNumber(row.total_nodes)}</td>
                  <td className={cell}>
                    {formatNumber(Math.round(row.average_nps))}
                  </td>
                </>
              )}
              <td className={`${cell} text-gray-500`}>
                {row.moves > 0 && `${Math.round(row.average_think_time_ms)} ms`}
              </td>
              {analyzed &&
                (row.analyzed_moves > 0 ? (
                  <>
                    <td className={`${cell} font-medium`}>
                      {row.accuracy.toFixed(1)}%
                    </td>
                    <td className={cell}>{Math.round(row.average_cp_loss)}</td>
                    <td className="py-2 text-right font-mono text-xs">
                      <span className={CLASSIFICATIONS.inaccuracy.className}>
                        {row.inaccuracies}
                      </span>
                      {' / '}
                      <span className={CLASSIFICATIONS.mistake.className}>
                        {row.mistakes}
                      </span>
                      {' / '}
                      <span className={CLASSIFICATIONS.blunder.className}>
                        {row.blunders}
                      </span>
                    </td>
                  </>
                ) : (
                  <td
                    colSpan={3}
                    className="py-2 text-right text-xs text-gray-500"
                  >
                    Not analyzed yet
                  </td>
                ))}
            </tr>
          ))}
        </tbody>
      </table>
      {analyzed && (
        <p className="mt-2 text-xs text-gray-500">
          Accuracy, ACPL and errors cover the{' '}
          {rows.reduce((sum, row) => sum + row.analyzed_moves, 0)} moves
          Stockfish has analyzed so far.
        </p>
      )}
    </div>
  )
}
