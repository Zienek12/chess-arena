import dynamic from 'next/dynamic'
const ChessViewer = dynamic(() => import('../components/ChessViewer'), {
  ssr: false,
})

export default function Home() {
  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-4xl mx-auto">
        <h1 className="text-2xl font-semibold mb-4">Chess Arena</h1>
        <p className="mb-4 text-sm text-gray-600">
          Play as White against the Rust engine.
        </p>
        <ChessViewer />
      </div>
    </div>
  )
}
