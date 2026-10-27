import Link from 'next/link'
import { useRouter } from 'next/router'
import { useState } from 'react'
import Layout from '../../components/Layout'
import {
  BackendUnavailable,
  Button,
  Card,
  ErrorMessage,
  StatusBadge,
} from '../../components/ui'
import {
  apiPost,
  isActive,
  useEngineNames,
  useTournament,
  useTournamentGames,
} from '../../lib/api'

function describeLimit(limit = {}) {
  if (limit.movetime_ms) return `${limit.movetime_ms} ms per move`
  if (limit.depth) return `depth ${limit.depth}`
  if (limit.nodes) return `${limit.nodes.toLocaleString()} nodes`
  return ''
}

function Stat({ label, value }) {
  return (
    <div>
      <p className="text-xs uppercase tracking-wide text-gray-500">{label}</p>
      <p className="text-xl font-semibold">{value}</p>
    </div>
  )
}

function ProgressBar({ progress }) {
  const total = progress.total || 1
  const segments = [
    ['completed', 'bg-emerald-500'],
    ['running', 'bg-amber-400'],
    ['failed', 'bg-rose-500'],
    ['cancelled', 'bg-gray-400'],
  ]
  return (
    <div className="flex h-2 overflow-hidden rounded-full bg-gray-100">
      {segments.map(([key, color]) => (
        <div
          key={key}
          className={color}
          style={{ width: `${(progress[key] / total) * 100}%` }}
        />
      ))}
    </div>
  )
}

function GamesTable({ games, names }) {
  if (!games) {
    return <p className="text-sm text-gray-500">Loading games...</p>
  }
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="text-xs uppercase text-gray-500">
          <tr>
            <th className="py-2 pr-3">#</th>
            <th className="py-2 pr-3">White</th>
            <th className="py-2 pr-3">Black</th>
            <th className="py-2 pr-3">Result</th>
            <th className="py-2 pr-3">Termination</th>
            <th className="py-2 pr-3 text-right">Moves</th>
            <th className="py-2">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {games.map((game) => (
            <tr key={game.id} className="hover:bg-gray-50">
              <td className="py-2 pr-3">
                <Link
                  href={`/games/${game.id}`}
                  className="font-medium text-sky-700 hover:underline"
                >
                  {game.game_number}
                </Link>
              </td>
              <td className="py-2 pr-3">{names[game.white_engine_id]}</td>
              <td className="py-2 pr-3">{names[game.black_engine_id]}</td>
              <td className="py-2 pr-3 font-mono">{game.result || '–'}</td>
              <td className="py-2 pr-3 text-gray-600">
                {game.termination.replaceAll('_', ' ')}
              </td>
              <td className="py-2 pr-3 text-right">
                {Math.ceil(game.ply / 2)}
              </td>
              <td className="py-2">
                <StatusBadge status={game.status} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export default function TournamentPage() {
  const { query } = useRouter()
  const { data: tournament, error, mutate } = useTournament(query.id)
  const { data: games } = useTournamentGames(query.id, tournament?.status)
  const names = useEngineNames()
  const [cancelError, setCancelError] = useState(null)

  async function cancel() {
    if (
      !window.confirm(
        'Cancel this tournament? Running games stop at their next move.'
      )
    ) {
      return
    }
    try {
      await apiPost(`/api/tournaments/${query.id}/cancel`)
      await mutate()
    } catch (err) {
      setCancelError(err)
    }
  }

  if (error) {
    return (
      <Layout title="Tournament">
        {error.status === 404 ? (
          <p>Tournament not found.</p>
        ) : (
          <BackendUnavailable error={error} />
        )}
      </Layout>
    )
  }
  if (!tournament) {
    return (
      <Layout title="Tournament">
        <p className="text-sm text-gray-500">Loading...</p>
      </Layout>
    )
  }

  const { progress, score } = tournament
  const engineA = names[tournament.engine_a_id] || 'Engine A'
  const engineB = names[tournament.engine_b_id] || 'Engine B'
  const played = score.engine_a_wins + score.engine_b_wins + score.draws

  return (
    <Layout title={tournament.name}>
      <Link
        href="/tournaments"
        className="text-sm text-gray-500 hover:text-gray-800"
      >
        ← Tournaments
      </Link>
      <div className="mb-6 mt-2 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-3 text-2xl font-semibold">
            {tournament.name} <StatusBadge status={tournament.status} />
          </h1>
          <p className="mt-1 text-sm text-gray-600">
            {tournament.game_count} games · {describeLimit(tournament.limit)} ·{' '}
            {tournament.cpus} CPU, {tournament.memory_mb} MB per engine
          </p>
        </div>
        {isActive(tournament.status) && (
          <Button variant="danger" onClick={cancel}>
            Cancel tournament
          </Button>
        )}
      </div>
      <ErrorMessage error={cancelError} />

      <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
        <Card title="Score">
          <div className="flex items-center justify-between gap-4">
            <div className="text-center">
              <p className="text-sm text-gray-600">{engineA}</p>
              <p className="text-3xl font-semibold">{score.engine_a_points}</p>
            </div>
            <p className="text-gray-400">–</p>
            <div className="text-center">
              <p className="text-sm text-gray-600">{engineB}</p>
              <p className="text-3xl font-semibold">{score.engine_b_points}</p>
            </div>
          </div>
          <p className="mt-4 text-center text-sm text-gray-500">
            {score.engine_a_wins} wins · {score.draws} draws ·{' '}
            {score.engine_b_wins} losses
            {played > 0 &&
              ` · ${engineA} scored ${Math.round(
                (score.engine_a_points / played) * 100
              )}%`}
          </p>
        </Card>
        <Card title="Progress">
          <ProgressBar progress={progress} />
          <div className="mt-4 grid grid-cols-4 gap-2">
            <Stat
              label="Done"
              value={`${progress.completed}/${progress.total}`}
            />
            <Stat label="Running" value={progress.running} />
            <Stat label="Queued" value={progress.queued} />
            <Stat label="Failed" value={progress.failed} />
          </div>
        </Card>
      </div>

      <Card title="Games" className="mt-6">
        <GamesTable games={games} names={names} />
      </Card>
    </Layout>
  )
}
