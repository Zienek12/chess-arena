import { useEffect, useMemo, useRef, useState } from 'react'
import { Chess, DEFAULT_POSITION } from 'chess.js'
import ResponsiveBoard from './ResponsiveBoard'
import { Button, inputClass } from './ui'
import { WS_URL } from '../lib/api'
import { applyUciMove, boardHighlights, legalTargets } from '../lib/chess'

const PLAYER = 'w'

export default function ChessViewer() {
  const chessRef = useRef(new Chess())
  const wsRef = useRef(null)
  const [fen, setFen] = useState(DEFAULT_POSITION)
  const [lastMove, setLastMove] = useState(null)
  const [selected, setSelected] = useState(null)
  const [connected, setConnected] = useState(false)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('Connecting to engine...')
  const [engineVersions, setEngineVersions] = useState([])
  const [selectedEngine, setSelectedEngine] = useState('')

  // A fresh game built from the FEN drives rendering; chessRef keeps the
  // authoritative history and is only touched in event handlers.
  const board = useMemo(() => new Chess(fen), [fen])
  const canMove =
    connected && !busy && board.turn() === PLAYER && !board.isGameOver()
  const targets = useMemo(
    () => (canMove ? legalTargets(board, selected) : []),
    [board, selected, canMove]
  )

  useEffect(() => {
    function handleServerMessage(data) {
      if (data.type === 'state') {
        const game = new Chess()
        for (const notation of data.moves || []) {
          applyUciMove(game, notation)
        }
        chessRef.current = game
        setFen(game.fen())
        setLastMove(null)
        setSelected(null)
        setBusy(false)
        setEngineVersions(data.engines || [])
        setSelectedEngine(data.engine || '')
        setMessage(`Your turn against ${data.engine || 'engine'}`)
      } else if (data.type === 'move') {
        const move = {
          from: data.from,
          to: data.to,
          promotion: data.promotion || undefined,
        }
        chessRef.current.move(move)
        setFen(chessRef.current.fen())
        setLastMove(move)
        if (data.turn === PLAYER) {
          setBusy(false)
          setMessage('Your turn')
        } else {
          setMessage('Engine is thinking...')
        }
      } else if (data.type === 'game_over') {
        setBusy(false)
        setMessage(data.message ? `Game over: ${data.message}` : 'Game over')
      } else if (data.type === 'error') {
        setBusy(false)
        setMessage(data.message || 'The move was rejected')
      }
    }

    const ws = new WebSocket(WS_URL)
    wsRef.current = ws
    ws.addEventListener('open', () => {
      setConnected(true)
      setMessage('Your turn')
    })
    ws.addEventListener('message', (event) => {
      try {
        handleServerMessage(JSON.parse(event.data))
      } catch {
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

  function sendMove(from, to) {
    const move = legalTargets(board, from).find(
      (candidate) => candidate.to === to
    )
    if (!move) {
      return false
    }
    // Promotions always choose a queen.
    const promotion = move.promotion ? 'q' : ''
    wsRef.current.send(JSON.stringify({ type: 'move', from, to, promotion }))
    setSelected(null)
    setBusy(true)
    setMessage('Engine is thinking...')
    return true
  }

  function handleSquareClick(square) {
    if (!canMove) {
      return
    }
    if (selected && targets.some((move) => move.to === square)) {
      sendMove(selected, square)
      return
    }
    const piece = board.get(square)
    if (piece && piece.color === PLAYER && square !== selected) {
      setSelected(square)
    } else {
      setSelected(null)
    }
  }

  function handlePieceDrop(sourceSquare, targetSquare) {
    if (canMove) {
      sendMove(sourceSquare, targetSquare)
    }
    // The board updates when the server confirms the move.
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
    <div className="space-y-4">
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
          className={`${inputClass} w-auto`}
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
        <Button disabled={!connected || busy} onClick={() => resetGame()}>
          New game
        </Button>
      </div>
      <ResponsiveBoard
        position={fen}
        arePiecesDraggable={canMove}
        isDraggablePiece={({ piece }) => piece[0] === PLAYER}
        onPieceDragBegin={(_, square) => setSelected(square)}
        onPieceDrop={handlePieceDrop}
        onSquareClick={handleSquareClick}
        customSquareStyles={boardHighlights({
          lastMove,
          selected: canMove ? selected : null,
          targets,
        })}
      />
      <p className="text-xs text-gray-500">
        Click a piece to see where it can move, then click a highlighted square,
        or drag the piece.
      </p>
    </div>
  )
}
