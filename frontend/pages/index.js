import dynamic from 'next/dynamic'
import { useEffect, useState } from 'react'

const ChessViewer = dynamic(() => import('../components/ChessViewer'), { ssr: false })

export default function Home() {
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    // nothing here; ChessViewer will open the WebSocket
  }, [])

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-2xl font-semibold mb-4">Chess Arena — Frontend Demo</h1>
        <p className="mb-4 text-sm text-gray-600">Live demo connects to backend WS at <code>ws://localhost:8080/ws</code></p>
        <ChessViewer />
      </div>
    </div>
  )
}
