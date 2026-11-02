import { formatNumber } from '../lib/chess'

// Search telemetry per engine: rows from summarizeTelemetry or the API.
export default function EngineTelemetryTable({ rows, names }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="text-xs uppercase text-gray-500">
          <tr>
            <th className="py-2 pr-3 text-left">Engine</th>
            <th className="py-2 pr-3 text-right">Moves</th>
            <th className="py-2 pr-3 text-right">Avg depth</th>
            <th className="py-2 pr-3 text-right">Max depth</th>
            <th className="py-2 pr-3 text-right">Avg nodes</th>
            <th className="py-2 pr-3 text-right">Total nodes</th>
            <th className="py-2 pr-3 text-right">Avg NPS</th>
            <th className="py-2 text-right">Avg time</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {rows.map((row, index) => (
            <tr key={`${row.engine_id}-${index}`}>
              <td className="py-2 pr-3 font-medium">
                {names[row.engine_id] || 'Engine'}
              </td>
              <td className="py-2 pr-3 text-right">{row.moves}</td>
              {row.moves_with_info === 0 ? (
                <td
                  colSpan={5}
                  className="py-2 pr-3 text-right text-xs text-gray-500"
                >
                  {row.moves === 0 ? '' : 'No search telemetry reported'}
                </td>
              ) : (
                <>
                  <td className="py-2 pr-3 text-right">
                    {row.average_depth.toFixed(1)}
                  </td>
                  <td className="py-2 pr-3 text-right">{row.max_depth}</td>
                  <td className="py-2 pr-3 text-right">
                    {formatNumber(Math.round(row.average_nodes))}
                  </td>
                  <td className="py-2 pr-3 text-right">
                    {formatNumber(row.total_nodes)}
                  </td>
                  <td className="py-2 pr-3 text-right">
                    {formatNumber(Math.round(row.average_nps))}
                  </td>
                </>
              )}
              <td className="py-2 text-right text-gray-500">
                {row.moves > 0 && `${Math.round(row.average_think_time_ms)} ms`}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
