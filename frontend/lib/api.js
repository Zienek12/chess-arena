import useSWR from 'swr'
import { useLive } from './live'

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

const TOKEN_KEY = 'chess-arena-api-token'

// The API token is kept in this browser only; storage can be unavailable.
export function getApiToken() {
  try {
    return window.localStorage.getItem(TOKEN_KEY) || ''
  } catch {
    return ''
  }
}

export function setApiToken(token) {
  try {
    if (token) {
      window.localStorage.setItem(TOKEN_KEY, token)
    } else {
      window.localStorage.removeItem(TOKEN_KEY)
    }
  } catch {
    // The token then lasts until the page is reloaded.
  }
}

export async function apiFetch(path, { method = 'GET', body } = {}) {
  const headers = {}
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  const token = typeof window === 'undefined' ? '' : getApiToken()
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }
  const response = await fetch(API_URL + path, {
    method,
    headers,
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

// While live updates are connected, polling is only a slow safety net.
const LIVE_FALLBACK_MS = 30000

// Polls every interval milliseconds while the resource is still in progress.
function refreshWhileActive(interval, live) {
  const every = live ? LIVE_FALLBACK_MS : interval
  return (data) => (data && isActive(data.status) ? every : 0)
}

export function useEngines() {
  return useSWR('/api/engines', apiFetch)
}

export function usePositionSets() {
  return useSWR('/api/position-sets', apiFetch)
}

export function useTournaments() {
  const live = useLive()
  return useSWR('/api/tournaments', apiFetch, {
    refreshInterval: live ? LIVE_FALLBACK_MS : 5000,
  })
}

export function useTournament(id) {
  const live = useLive()
  return useSWR(id ? `/api/tournaments/${id}` : null, apiFetch, {
    refreshInterval: refreshWhileActive(2000, live),
  })
}

export function useTournamentGames(id, tournamentStatus) {
  const live = useLive()
  return useSWR(id ? `/api/tournaments/${id}/games` : null, apiFetch, {
    refreshInterval: isActive(tournamentStatus)
      ? live
        ? LIVE_FALLBACK_MS
        : 2000
      : 0,
  })
}

// Games are reloaded after they finish too, until their analysis is done.
export function useGame(id) {
  const live = useLive()
  return useSWR(id ? `/api/games/${id}` : null, apiFetch, {
    refreshInterval: (game) => {
      if (!game) return 0
      const analyzing =
        game.status === 'completed' && game.analysis?.status !== 'completed'
      if (!isActive(game.status) && !analyzing) return 0
      if (live) return LIVE_FALLBACK_MS
      return analyzing ? 5000 : 1000
    },
  })
}

export function useLeaderboard() {
  const live = useLive()
  return useSWR('/api/leaderboard', apiFetch, {
    refreshInterval: live ? LIVE_FALLBACK_MS : 10000,
  })
}

// Whether writes need an API token, and whether the stored one is valid.
export function useAuth() {
  return useSWR('/api/auth', apiFetch)
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

// Names a game's starting position: its opening when the position set names
// it, otherwise its number in the set.
export function openingLabel(game) {
  return game.opening || `Position ${game.position_number}`
}

// Short form of a Docker image ID ("sha256:…").
export function shortImageId(imageId) {
  return imageId ? imageId.replace(/^sha256:/, '').slice(0, 12) : ''
}

export function engineLabel(engine) {
  return `${engine.name} ${engine.version}`
}
