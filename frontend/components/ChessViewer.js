import { useEffect, useRef, useState } from 'react'
import { Chess } from 'chess.js'
import { Chessboard } from 'react-chessboard'

export default function ChessViewer() {
  const chessRef = useRef(new Chess())
  const [fen, setFen] = useState(chessRef.current.fen())
  const wsRef = useRef(null)

  useEffect(() => {
    const ws = new WebSocket('ws://localhost:8080/ws')
    wsRef.current = ws
    ws.addEventListener('open', () => console.log('WS open'))
    ws.addEventListener('message', (ev) => {
      try {
        const data = JSON.parse(ev.data)
        if (data.type === 'move') {
          const { from, to } = data
          chessRef.current.move({ from, to })
          setFen(chessRef.current.fen())
        } else if (data.type === 'reset') {
          chessRef.current.reset()
          setFen(chessRef.current.fen())
        }
      } catch (e) {
        console.error('Invalid WS data', e)
      }
    })
    ws.addEventListener('close', () => console.log('WS closed'))
    return () => ws.close()
  }, [])

  return (
    <div className="bg-white rounded shadow p-4">
      <Chessboard position={fen} boardWidth={560} />
    </div>
  )
}
