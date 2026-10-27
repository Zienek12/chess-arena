import { useEffect, useMemo, useRef, useState } from 'react'
import ResponsiveBoard from '../ResponsiveBoard'
import { Button } from '../ui'
import {
  boardHighlights,
  formatNumber,
  formatScore,
  parseUci,
  sanMoves,
} from '../../lib/chess'

// Steps through a stored game. While the game is running and the user has not
// picked a move, the board follows the latest move.
export default function GameReplay({ game }) {
  const moves = game.moves
  const [chosenPly, setChosenPly] = useState(null)
  const ply = chosenPly ?? moves.length
  const startsWithBlack = game.start_fen.split(' ')[1] === 'b'
  const san = useMemo(
    () =>
      sanMoves(
        game.start_fen,
        moves.map((move) => move.uci)
      ),
    [game.start_fen, moves]
  )

  const fen = ply === 0 ? game.start_fen : moves[ply - 1].fen_after
  const lastMove = ply === 0 ? null : parseUci(moves[ply - 1].uci)

  function goTo(next) {
    const clamped = Math.max(0, Math.min(moves.length, next))
    setChosenPly(clamped === moves.length ? null : clamped)
  }

  // Keyboard navigation reads the current ply through a ref, so the listener
  // is registered once.
  const navigationRef = useRef({ ply, goTo })
  useEffect(() => {
    navigationRef.current = { ply, goTo }
  })
  useEffect(() => {
    function onKey(event) {
      if (event.target.closest?.('input, textarea, select')) return
      const { ply: current, goTo: go } = navigationRef.current
      if (event.key === 'ArrowLeft') go(current - 1)
      if (event.key === 'ArrowRight') go(current + 1)
      if (event.key === 'Home') go(0)
      if (event.key === 'End') go(Number.MAX_SAFE_INTEGER)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <div className="space-y-3">
        <ResponsiveBoard
          position={fen}
          arePiecesDraggable={false}
          animationDuration={150}
          customSquareStyles={boardHighlights({ lastMove })}
        />
        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            onClick={() => goTo(0)}
            disabled={ply === 0}
            aria-label="First move"
          >
            «
          </Button>
          <Button
            variant="secondary"
            onClick={() => goTo(ply - 1)}
            disabled={ply === 0}
            aria-label="Previous move"
          >
            ‹
          </Button>
          <Button
            variant="secondary"
            onClick={() => goTo(ply + 1)}
            disabled={ply === moves.length}
            aria-label="Next move"
          >
            ›
          </Button>
          <Button
            variant="secondary"
            onClick={() => goTo(moves.length)}
            disabled={ply === moves.length}
            aria-label="Last move"
          >
            »
          </Button>
          <span className="ml-2 text-sm text-gray-500">
            {ply} / {moves.length}
            {chosenPly === null && game.status === 'running' && ' · live'}
          </span>
        </div>
        <p className="text-xs text-gray-500">
          Use ← and → to step through the moves.
        </p>
      </div>

      <div className="max-h-[640px] overflow-y-auto rounded border border-gray-200 bg-white">
        <table className="w-full text-sm">
          <thead className="sticky top-0 bg-gray-50 text-xs uppercase text-gray-500">
            <tr>
              <th className="px-3 py-2 text-left">Move</th>
              <th className="px-3 py-2 text-right">Eval</th>
              <th className="px-3 py-2 text-right">Depth</th>
              <th className="px-3 py-2 text-right">Nodes</th>
              <th className="px-3 py-2 text-right">Time</th>
            </tr>
          </thead>
          <tbody>
            {moves.map((move, index) => {
              const moverIsWhite = (index % 2 === 0) !== startsWithBlack
              const number =
                Math.floor((index + (startsWithBlack ? 1 : 0)) / 2) + 1
              const selected = index + 1 === ply
              return (
                <tr
                  key={move.ply}
                  onClick={() => goTo(index + 1)}
                  className={`cursor-pointer ${
                    selected ? 'bg-amber-100' : 'hover:bg-gray-50'
                  }`}
                >
                  <td className="px-3 py-1.5 font-mono">
                    <span className="inline-block w-12 text-gray-400">
                      {number}
                      {moverIsWhite ? '.' : '...'}
                    </span>
                    {san[index]}
                  </td>
                  <td className="px-3 py-1.5 text-right font-mono">
                    {formatScore(move.info, moverIsWhite)}
                  </td>
                  <td className="px-3 py-1.5 text-right">
                    {move.info.depth || ''}
                  </td>
                  <td className="px-3 py-1.5 text-right">
                    {formatNumber(move.info.nodes)}
                  </td>
                  <td className="px-3 py-1.5 text-right text-gray-500">
                    {move.think_time_ms !== undefined
                      ? `${move.think_time_ms} ms`
                      : ''}
                  </td>
                </tr>
              )
            })}
            {moves.length === 0 && (
              <tr>
                <td colSpan={5} className="px-3 py-4 text-center text-gray-500">
                  No moves yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
