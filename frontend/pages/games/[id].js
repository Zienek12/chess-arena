import Link from 'next/link'
import { useRouter } from 'next/router'
import Layout from '../../components/Layout'
import GameReplay from '../../components/games/GameReplay'
import { BackendUnavailable, Card, StatusBadge } from '../../components/ui'
import { API_URL, useEngineNames, useGame } from '../../lib/api'

export default function GamePage() {
  const { query } = useRouter()
  const { data: game, error } = useGame(query.id)
  const names = useEngineNames()

  if (error) {
    return (
      <Layout title="Game">
        {error.status === 404 ? (
          <p>Game not found.</p>
        ) : (
          <BackendUnavailable error={error} />
        )}
      </Layout>
    )
  }
  if (!game) {
    return (
      <Layout title="Game">
        <p className="text-sm text-gray-500">Loading...</p>
      </Layout>
    )
  }

  const white = names[game.white_engine_id] || 'White'
  const black = names[game.black_engine_id] || 'Black'

  return (
    <Layout title={`Game ${game.game_number}`}>
      <Link
        href={`/tournaments/${game.tournament_id}`}
        className="text-sm text-gray-500 hover:text-gray-800"
      >
        ← Tournament
      </Link>
      <div className="mb-6 mt-2 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-3 text-2xl font-semibold">
            Game {game.game_number} <StatusBadge status={game.status} />
          </h1>
          <p className="mt-1 text-sm text-gray-600">
            <span className="font-medium text-gray-900">{white}</span> (White)
            vs <span className="font-medium text-gray-900">{black}</span>{' '}
            (Black)
            {game.result && (
              <>
                {' · '}
                <span className="font-mono">{game.result}</span>
                {game.termination &&
                  ` by ${game.termination.replaceAll('_', ' ')}`}
              </>
            )}
          </p>
        </div>
        {game.pgn && (
          <a
            href={`${API_URL}/api/games/${game.id}/pgn`}
            className="rounded border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-800 hover:bg-gray-50"
          >
            Download PGN
          </a>
        )}
      </div>
      <Card>
        <GameReplay game={game} />
      </Card>
    </Layout>
  )
}
