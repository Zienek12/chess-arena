import { useEffect, useRef, useState } from 'react'
import { Chess, DEFAULT_POSITION } from 'chess.js'
import { Chessboard } from 'react-chessboard'

export default function ChessViewer() {
  const chessRef = useRef(new Chess())
  const [fen, setFen] = useState(DEFAULT_POSITION)
  const [connected, setConnected] = useState(false)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('Connecting to engine...')
  const [engineVersions, setEngineVersions] = useState([])
  const [selectedEngine, setSelectedEngine] = useState('')
  const wsRef = useRef(null)

  useEffect(() => {
    const ws = new WebSocket('ws://localhost:8080/ws')
    wsRef.current = ws
    ws.addEventListener('open', () => {
      setConnected(true)
      setMessage('Your turn')
    })
    ws.addEventListener('message', (ev) => {
      try {
        const data = JSON.parse(ev.data)
        if (data.type === 'state') {
          const game = new Chess()
          for (const notation of data.moves || []) {
            applyUciMove(game, notation)
          }
          chessRef.current = game
          setFen(game.fen())
          setBusy(false)
          setEngineVersions(data.engines || [])
          setSelectedEngine(data.engine || '')
          setMessage(`Your turn against ${data.engine || 'engine'}`)
        } else if (data.type === 'move') {
          chessRef.current.move({
            from: data.from,
            to: data.to,
            promotion: data.promotion || undefined,
          })
          setFen(chessRef.current.fen())
          if (data.turn === 'w') {
            setBusy(false)
            setMessage('Your turn')
          } else {
            setMessage('Engine is thinking...')
          }
        } else if (data.type === 'game_over') {
          setBusy(false)
          setMessage('Game over')
        } else if (data.type === 'error') {
          setBusy(false)
          setMessage(data.message || 'The move was rejected')
        }
      } catch (e) {
        setBusy(false)
        setMessage('Invalid server response')
      }
    })
    ws.addEventListener('close', () => {
      setConnected(false)
      setBusy(false)
      setMessage('Disconnected from backend')
    })
    ws.addEventListener('error', () => {
      setConnected(false)
      setBusy(false)
      setMessage('Could not connect to backend')
    })
    return () => {
      ws.close()
      wsRef.current = null
    }
  }, [])

  function handlePieceDrop(sourceSquare, targetSquare, piece) {
    if (!connected || busy || chessRef.current.turn() !== 'w') {
      return false
    }

    const movingPiece = chessRef.current.get(sourceSquare)
    if (!movingPiece || movingPiece.color !== 'w') {
      return false
    }

    const promotion =
      movingPiece.type === 'p' && targetSquare[1] === '8' ? 'q' : ''
    wsRef.current.send(
      JSON.stringify({
        type: 'move',
        from: sourceSquare,
        to: targetSquare,
        promotion,
      })
    )
    setBusy(true)
    setMessage('Engine is thinking...')
    return false
  }

  function resetGame(engine = selectedEngine) {
    if (!connected || busy || !wsRef.current) {
      return
    }
    setBusy(true)
    setMessage('Starting a new game...')
    wsRef.current.send(JSON.stringify({ type: 'reset', engine }))
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3 text-sm">
        <span className={connected ? 'text-emerald-600' : 'text-rose-600'}>
          {connected ? 'Connected' : 'Offline'}
        </span>
        <span className="text-gray-600">{message}</span>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <label className="text-sm text-gray-700" htmlFor="engine-version">
          Engine
        </label>
        <select
          id="engine-version"
          className="rounded border border-gray-300 bg-white px-3 py-2 text-sm"
          value={selectedEngine}
          disabled={!connected || busy || engineVersions.length === 0}
          onChange={(event) => {
            setSelectedEngine(event.target.value)
            resetGame(event.target.value)
          }}
        >
          {engineVersions.length === 0 && <option value="">Unavailable</option>}
          {engineVersions.map((engine) => (
            <option key={engine} value={engine}>
              {engine}
            </option>
          ))}
        </select>
        <button
          type="button"
          className="rounded bg-gray-900 px-3 py-2 text-sm text-white disabled:cursor-not-allowed disabled:opacity-50"
          disabled={!connected || busy}
          onClick={() => resetGame()}
        >
          New game
        </button>
      </div>
      <Chessboard
        position={fen}
        boardWidth={560}
        arePiecesDraggable={connected && !busy}
        onPieceDrop={handlePieceDrop}
      />
    </div>
  )
}

function applyUciMove(game, notation) {
  game.move({
    from: notation.slice(0, 2),
    to: notation.slice(2, 4),
    promotion: notation[4] || undefined,
  })
}
