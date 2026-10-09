import { useEffect, useRef, useState } from 'react'
import { Chess } from 'chess.js'
import { Chessboard } from 'react-chessboard'

export default function ChessViewer() {
  const chessRef = useRef(new Chess())
  const [fen, setFen] = useState(chessRef.current.fen())
  const [connected, setConnected] = useState(false)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('Connecting to engine...')
  const [engine, setEngine] = useState('')
  const [engines, setEngines] = useState([])
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
          setEngine(data.engine || '')
          setEngines(data.engines || [])
          setBusy(false)
          setMessage('Your turn')
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

    const promotion = movingPiece.type === 'p' && targetSquare[1] === '8' ? 'q' : ''
    wsRef.current.send(JSON.stringify({
      type: 'move',
      from: sourceSquare,
      to: targetSquare,
      promotion,
    }))
    setBusy(true)
    setMessage('Engine is thinking...')
    return false
  }

  function handleEngineChange(event) {
    const nextEngine = event.target.value
    setEngine(nextEngine)
    setBusy(true)
    setMessage('Resetting game...')
    wsRef.current?.send(JSON.stringify({ type: 'reset', engine: nextEngine }))
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between text-sm">
        <span className={connected ? 'text-emerald-600' : 'text-rose-600'}>
          {connected ? 'Connected' : 'Offline'}
        </span>
        <span className="text-gray-600">{message}</span>
      </div>
      {engines.length > 0 && (
        <label className="flex items-center gap-2 text-sm text-gray-700">
          <span>Engine</span>
          <select
            className="rounded border border-gray-300 bg-white px-2 py-1"
            value={engine}
            onChange={handleEngineChange}
            disabled={!connected || busy}
          >
            {engines.map((version) => (
              <option key={version} value={version}>{version}</option>
            ))}
          </select>
        </label>
      )}
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
