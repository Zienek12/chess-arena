import Layout from '../components/Layout'
import { BackendUnavailable, Card } from '../components/ui'
import { engineLabel, useLeaderboard } from '../lib/api'
import { formatNumber } from '../lib/chess'

const cell = 'py-2 pr-3 text-right'

function signed(value) {
  const rounded = Math.round(value)
  return `${rounded > 0 ? '+' : ''}${rounded}`
}

export default function LeaderboardPage() {
  const { data: entries, error } = useLeaderboard()

  let body
  if (error) {
    body = <BackendUnavailable error={error} />
  } else if (!entries) {
    body = <p className="text-sm text-gray-500">Loading...</p>
  } else if (entries.length === 0) {
    body = (
      <p className="text-sm text-gray-600">
        No finished games yet. Ratings appear once tournaments have results.
      </p>
    )
  } else {
    const anchored = entries.some((entry) => entry.elo !== undefined)
    body = (
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="text-xs uppercase text-gray-500">
            <tr>
              <th className="py-2 pr-3 text-left">#</th>
              <th className="py-2 pr-3 text-left">Engine</th>
              {anchored && (
                <th
                  className={cell}
                  title="Rating on the scale of the anchors' declared Elo"
                >
                  Elo (approx.)
                </th>
              )}
              <th className={cell} title="Relative Elo; ratings average to 0">
                Rating
              </th>
              <th className={cell}>Games</th>
              <th className={cell}>W / D / L</th>
              <th className={cell}>Score</th>
              <th className={cell}>Avg depth</th>
              <th className={cell}>Avg NPS</th>
              <th className={cell} title="Average accuracy per analyzed move">
                Accuracy
              </th>
              <th
                className={cell}
                title="Average centipawn loss per analyzed move"
              >
                ACPL
              </th>
              <th
                className="py-2 text-right"
                title="Blunders per 100 analyzed moves"
              >
                Blunders
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {entries.map((entry, index) => (
              <tr key={entry.id}>
                <td className="py-2 pr-3 text-gray-500">{index + 1}</td>
                <td className="py-2 pr-3">
                  <p className="font-medium">{engineLabel(entry)}</p>
                  <p className="font-mono text-xs text-gray-500">
                    {entry.image}
                  </p>
                  {entry.anchor_elo && (
                    <p className="text-xs text-violet-700">
                      Anchor: declared {entry.anchor_elo} Elo
                    </p>
                  )}
                </td>
                {anchored && (
                  <td className={`${cell} font-mono font-semibold`}>
                    {Math.round(entry.elo)}
                  </td>
                )}
                <td className={`${cell} font-mono`}>{signed(entry.rating)}</td>
                <td className={cell}>{entry.games}</td>
                <td className={`${cell} font-mono text-xs`}>
                  {entry.wins} / {entry.draws} / {entry.losses}
                </td>
                <td className={cell}>
                  {((entry.points / entry.games) * 100).toFixed(1)}%
                </td>
                <td className={cell}>
                  {entry.average_depth ? entry.average_depth.toFixed(1) : '–'}
                </td>
                <td className={cell}>
                  {formatNumber(Math.round(entry.average_nps)) || '–'}
                </td>
                {entry.analyzed_moves > 0 ? (
                  <>
                    <td className={`${cell} font-medium`}>
                      {entry.accuracy.toFixed(1)}%
                    </td>
                    <td className={cell}>
                      {Math.round(entry.average_cp_loss)}
                    </td>
                    <td className="py-2 text-right">
                      {((entry.blunders / entry.analyzed_moves) * 100).toFixed(
                        1
                      )}
                    </td>
                  </>
                ) : (
                  <td
                    colSpan={3}
                    className="py-2 text-right text-xs text-gray-500"
                  >
                    Not analyzed yet
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  return (
    <Layout title="Leaderboard">
      <h1 className="mb-1 text-2xl font-semibold">Leaderboard</h1>
      <p className="mb-6 text-sm text-gray-600">
        Ratings are fitted to every finished game across all tournaments
        (Bradley-Terry model), so engines that never met are compared through
        common opponents. Move quality comes from Stockfish&apos;s analysis. The
        approximate Elo appears once an anchor has played: a Stockfish
        registered with UCI options{' '}
        <code>UCI_LimitStrength=true, UCI_Elo=N</code>. Ratings are then shifted
        so the anchors average their declared Elo. Stockfish calibrates that
        scale at longer time controls, so treat it as an estimate.
      </p>
      <Card>{body}</Card>
    </Layout>
  )
}
