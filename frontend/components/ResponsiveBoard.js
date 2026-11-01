import dynamic from 'next/dynamic'
import { useEffect, useRef, useState } from 'react'

// react-chessboard needs the browser, so it is not rendered on the server.
const Chessboard = dynamic(
  () => import('react-chessboard').then((module) => module.Chessboard),
  { ssr: false }
)

const MAX_WIDTH = 560

// Chessboard that fills its container, up to MAX_WIDTH pixels.
export default function ResponsiveBoard(props) {
  const containerRef = useRef(null)
  const [width, setWidth] = useState(MAX_WIDTH)

  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return undefined
    }
    const observer = new ResizeObserver(([entry]) => {
      setWidth(Math.min(MAX_WIDTH, Math.floor(entry.contentRect.width)))
    })
    observer.observe(container)
    return () => observer.disconnect()
  }, [])

  return (
    <div ref={containerRef} className="w-full" style={{ maxWidth: MAX_WIDTH }}>
      <Chessboard boardWidth={width} {...props} />
    </div>
  )
}
