import '../styles/globals.css'
import { LiveContext, useLiveUpdates } from '../lib/live'

export default function App({ Component, pageProps }) {
  const live = useLiveUpdates()
  return (
    <LiveContext.Provider value={live}>
      <Component {...pageProps} />
    </LiveContext.Provider>
  )
}
