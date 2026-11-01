import useSWR from 'swr'

export const API_URL = (
  process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
).replace(/\/$/, '')

export const WS_URL = `${API_URL.replace(/^http/, 'ws')}/ws`

export class ApiError extends Error {
  constructor(message, status) {
    super(message)
    this.status = status
  }
}

export async function apiFetch(path, { method = 'GET', body } = {}) {
  const response = await fetch(API_URL + path, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (response.status === 204) {
    return null
  }
  const data = await response.json().catch(() => null)
  if (!response.ok) {
    throw new ApiError(
      data?.error || `Request failed (${response.status})`,
      response.status
    )
  }
  return data
}

export function apiPost(path, body) {
  return apiFetch(path, { method: 'POST', body })
}

const ACTIVE_STATUSES = ['created', 'queued', 'running']

export function isActive(status) {
  return ACTIVE_STATUSES.includes(status)
}

// Polls every interval milliseconds while the resource is still in progress.
function refreshWhileActive(interval) {
  return (data) => (data && isActive(data.status) ? interval : 0)
}

export function useEngines() {
  return useSWR('/api/engines', apiFetch)
}

export function usePositionSets() {
  return useSWR('/api/position-sets', apiFetch)
}

export function useTournaments() {
  return useSWR('/api/tournaments', apiFetch, { refreshInterval: 5000 })
}

export function useTournament(id) {
  return useSWR(id ? `/api/tournaments/${id}` : null, apiFetch, {
    refreshInterval: refreshWhileActive(2000),
  })
}

export function useTournamentGames(id, tournamentStatus) {
  return useSWR(id ? `/api/tournaments/${id}/games` : null, apiFetch, {
    refreshInterval: isActive(tournamentStatus) ? 2000 : 0,
  })
}

export function useGame(id) {
  return useSWR(id ? `/api/games/${id}` : null, apiFetch, {
    refreshInterval: refreshWhileActive(1000),
  })
}

// Maps engine IDs to "name version" labels.
export function useEngineNames() {
  const { data } = useEngines()
  const names = {}
  for (const engine of data || []) {
    names[engine.id] = engineLabel(engine)
  }
  return names
}

export function engineLabel(engine) {
  return `${engine.name} ${engine.version}`
}
