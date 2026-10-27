import Head from 'next/head'
import Link from 'next/link'
import { useRouter } from 'next/router'

const NAVIGATION = [
  { href: '/', label: 'Play' },
  { href: '/tournaments', label: 'Tournaments' },
]

export default function Layout({ title, children }) {
  const { pathname } = useRouter()
  return (
    <div className="min-h-screen bg-gray-50 text-gray-900">
      <Head>
        <title>{title ? `${title} · Chess Arena` : 'Chess Arena'}</title>
      </Head>
      <header className="border-b border-gray-200 bg-white">
        <nav className="mx-auto flex max-w-6xl items-center gap-6 px-4 py-3">
          <Link href="/" className="font-semibold">
            Chess Arena
          </Link>
          {NAVIGATION.map((item) => {
            const active =
              item.href === '/'
                ? pathname === '/'
                : pathname.startsWith(item.href) ||
                  pathname.startsWith('/games')
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`text-sm ${
                  active
                    ? 'font-medium text-gray-900'
                    : 'text-gray-500 hover:text-gray-800'
                }`}
              >
                {item.label}
              </Link>
            )
          })}
        </nav>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-8">{children}</main>
    </div>
  )
}
