import Layout from '../components/Layout'
import ChessViewer from '../components/ChessViewer'

export default function Home() {
  return (
    <Layout title="Play">
      <div className="max-w-xl">
        <h1 className="mb-1 text-2xl font-semibold">Play against the engine</h1>
        <p className="mb-6 text-sm text-gray-600">You play White.</p>
        <ChessViewer />
      </div>
    </Layout>
  )
}
