import { createContext, useContext, useEffect, useState } from 'react'
import { mutate } from 'swr'
import { API_URL } from './api'

// Whether the live event stream is connected. Pages poll slowly while it is.
export const LiveContext = createContext(false)

export function useLive() {
  return useContext(LiveContext)
}

// Changes arrive in bursts (a move per second per game); reload each affected
// resource at most once per BATCH_MS.
const BATCH_MS = 300

// Resources to reload for a change notification from the backend.
function affectedKeys(event) {
  const keys = new Set()
  if (event.tournament_id) {
    keys.add(`/api/tournaments/${event.tournament_id}`)
    keys.add(`/api/tournaments/${event.tournament_id}/games`)
  }
  if (event.game_id) {
    keys.add(`/api/games/${event.game_id}`)
  }
  if (event.type === 'tournament' || event.type === 'game') {
    keys.add('/api/tournaments')
  }
  if (event.type === 'game' || event.type === 'analysis') {
    keys.add('/api/leaderboard')
  }
  return keys
}

// Subscribes to /api/events once for the whole app and revalidates the SWR
// resources each event affects.
export function useLiveUpdates() {
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    if (typeof EventSource === 'undefined') {
      return undefined
    }
    const source = new EventSource(`${API_URL}/api/events`)
    let pending = new Set()
    let timer = null

    function flush() {
      timer = null
      const keys = pending
      pending = new Set()
      if (keys.has('*')) {
        mutate(() => true)
        return
      }
      for (const key of keys) {
        mutate(key)
      }
    }

    source.onopen = () => setConnected(true)
    source.onerror = () => setConnected(false)
    source.onmessage = (message) => {
      let event
      try {
        event = JSON.parse(message.data)
      } catch {
        return
      }
      const keys = event.type === 'resync' ? ['*'] : affectedKeys(event)
      keys.forEach((key) => pending.add(key))
      timer ??= setTimeout(flush, BATCH_MS)
    }
    return () => {
      clearTimeout(timer)
      source.close()
    }
  }, [])

  return connected
}
