import Link from 'next/link'
import Layout from '../../components/Layout'
import EngineForm from '../../components/tournaments/EngineForm'
import PositionSetForm from '../../components/tournaments/PositionSetForm'
import TournamentForm from '../../components/tournaments/TournamentForm'
import { BackendUnavailable, Card, StatusBadge } from '../../components/ui'
import { useEngineNames, useEngines, useTournaments } from '../../lib/api'

function TournamentList() {
  const { data: tournaments, error } = useTournaments()
  const names = useEngineNames()
  if (error) {
    return <BackendUnavailable error={error} />
  }
  return (
    <Card title="Tournaments">
      {!tournaments ? (
        <p className="text-sm text-gray-500">Loading...</p>
      ) : tournaments.length === 0 ? (
        <p className="text-sm text-gray-500">No tournaments yet.</p>
      ) : (
        <ul className="divide-y divide-gray-100">
          {tournaments.map((tournament) => (
            <li key={tournament.id}>
              <Link
                href={`/tournaments/${tournament.id}`}
                className="flex flex-wrap items-center justify-between gap-2 py-3 hover:bg-gray-50"
              >
                <div>
                  <p className="text-sm font-medium">{tournament.name}</p>
                  <p className="text-xs text-gray-500">
                    {names[tournament.engine_a_id] || 'engine A'} vs{' '}
                    {names[tournament.engine_b_id] || 'engine B'} ·{' '}
                    {tournament.game_count} games ·{' '}
                    {new Date(tournament.created_at).toLocaleString()}
                  </p>
                </div>
                <StatusBadge status={tournament.status} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}

function EngineList() {
  const { data: engines } = useEngines()
  if (!engines || engines.length === 0) {
    return null
  }
  return (
    <ul className="mb-4 space-y-1 text-sm">
      {engines.map((engine) => (
        <li key={engine.id} className="flex justify-between gap-2">
          <span className="font-medium">
            {engine.name} {engine.version}
          </span>
          <code className="truncate text-xs text-gray-500">{engine.image}</code>
        </li>
      ))}
    </ul>
  )
}

export default function Tournaments() {
  return (
    <Layout title="Tournaments">
      <h1 className="mb-6 text-2xl font-semibold">Tournaments</h1>
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-5">
        <div className="space-y-6 lg:col-span-3">
          <Card title="New tournament">
            <TournamentForm />
          </Card>
          <TournamentList />
        </div>
        <div className="space-y-6 lg:col-span-2">
          <Card title="Engine versions">
            <EngineList />
            <EngineForm />
          </Card>
          <Card title="Import starting positions">
            <PositionSetForm />
          </Card>
        </div>
      </div>
    </Layout>
  )
}
