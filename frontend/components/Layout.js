import Head from 'next/head'
import Link from 'next/link'
import { useRouter } from 'next/router'
import { getApiToken, setApiToken, useAuth } from '../lib/api'
import { useLive } from '../lib/live'

const NAVIGATION = [
  { href: '/', label: 'Play' },
  { href: '/tournaments', label: 'Tournaments', also: ['/games'] },
  { href: '/leaderboard', label: 'Leaderboard' },
]

function isCurrent(item, pathname) {
  if (item.href === '/') {
    return pathname === '/'
  }
  return [item.href, ...(item.also || [])].some((prefix) =>
    pathname.startsWith(prefix)
  )
}

// Shows whether writes need the API token and lets the user enter it.
function TokenButton() {
  const { data: auth, mutate } = useAuth()
  if (!auth?.token_required) {
    return null
  }
  function edit() {
    const token = window.prompt(
      'API token (ARENA_API_TOKEN). Leave empty to forget it.',
      getApiToken()
    )
    if (token !== null) {
      setApiToken(token.trim())
      mutate()
    }
  }
  return (
    <button
      type="button"
      onClick={edit}
      className={`rounded border px-2 py-1 text-xs font-medium ${
        auth.authorized
          ? 'border-emerald-300 text-emerald-700 hover:bg-emerald-50'
          : 'border-amber-300 bg-amber-50 text-amber-800 hover:bg-amber-100'
      }`}
      title="Creating tournaments and engines requires the API token"
    >
      {auth.authorized ? 'Token set' : 'Enter API token'}
    </button>
  )
}

export default function Layout({ title, children }) {
  const { pathname } = useRouter()
  const live = useLive()
  return (
    <div className="min-h-screen bg-gray-50 text-gray-900">
      <Head>
        <title>{title ? `${title} · Chess Arena` : 'Chess Arena'}</title>
      </Head>
      <header className="border-b border-gray-200 bg-white">
        <nav className="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
          <Link href="/" className="font-semibold">
            Chess Arena
          </Link>
          {NAVIGATION.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={`text-sm ${
                isCurrent(item, pathname)
                  ? 'font-medium text-gray-900'
                  : 'text-gray-500 hover:text-gray-800'
              }`}
            >
              {item.label}
            </Link>
          ))}
          <div className="ml-auto flex items-center gap-3">
            <span
              className="flex items-center gap-1.5 text-xs text-gray-500"
              title={
                live
                  ? 'Updates arrive as they happen'
                  : 'Live updates unavailable; pages refresh periodically'
              }
            >
              <span
                className={`h-2 w-2 rounded-full ${
                  live ? 'bg-emerald-500' : 'bg-gray-300'
                }`}
              />
              {live ? 'Live' : 'Polling'}
            </span>
            <TokenButton />
          </div>
        </nav>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-8">{children}</main>
    </div>
  )
}
