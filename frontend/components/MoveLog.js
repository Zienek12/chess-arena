import { formatNumber, formatScore } from '../lib/chess'

// Moves of the current game; engine moves show their telemetry. Clicking an
// engine move selects it.
export default function MoveLog({ moves, selectedIndex, onSelect }) {
  return (
    <div className="max-h-80 overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-sm">
      <table className="w-full text-sm">
        <thead className="sticky top-0 bg-gray-50 text-xs uppercase text-gray-500">
          <tr>
            <th className="px-3 py-2 text-left">Move</th>
            <th className="px-3 py-2 text-right">Eval</th>
            <th className="px-3 py-2 text-right">Depth</th>
            <th className="px-3 py-2 text-right">Nodes</th>
            <th className="px-3 py-2 text-right">Time</th>
          </tr>
        </thead>
        <tbody>
          {moves.map((move, index) => {
            const isEngine = move.info !== undefined
            const selected = index === selectedIndex
            return (
              <tr
                key={index}
                onClick={isEngine ? () => onSelect(index) : undefined}
                className={
                  selected
                    ? 'bg-amber-100'
                    : isEngine
                    ? 'cursor-pointer hover:bg-gray-50'
                    : ''
                }
              >
                <td className="px-3 py-1.5 font-mono">
                  <span className="inline-block w-12 text-gray-400">
                    {move.number}
                    {move.color === 'w' ? '.' : '...'}
                  </span>
                  {move.san}
                </td>
                <td className="px-3 py-1.5 text-right font-mono">
                  {isEngine ? formatScore(move.info, move.color === 'w') : ''}
                </td>
                <td className="px-3 py-1.5 text-right">
                  {move.info?.depth || ''}
                </td>
                <td className="px-3 py-1.5 text-right">
                  {formatNumber(move.info?.nodes)}
                </td>
                <td className="px-3 py-1.5 text-right text-gray-500">
                  {isEngine && move.thinkTimeMs !== undefined
                    ? `${move.thinkTimeMs} ms`
                    : ''}
                </td>
              </tr>
            )
          })}
          {moves.length === 0 && (
            <tr>
              <td colSpan={5} className="px-3 py-4 text-center text-gray-500">
                No moves yet.
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}
