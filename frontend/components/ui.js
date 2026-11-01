// Small shared building blocks so pages share one look.

export const inputClass =
  'w-full rounded border border-gray-300 bg-white px-3 py-2 text-sm focus:border-gray-500 focus:outline-none disabled:bg-gray-100'

export function Button({ variant = 'primary', className = '', ...props }) {
  const variants = {
    primary: 'bg-gray-900 text-white hover:bg-gray-700',
    secondary: 'border border-gray-300 bg-white text-gray-800 hover:bg-gray-50',
    danger: 'bg-rose-600 text-white hover:bg-rose-500',
  }
  return (
    <button
      type="button"
      className={`rounded px-3 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50 ${variants[variant]} ${className}`}
      {...props}
    />
  )
}

export function Card({ title, actions, children, className = '' }) {
  return (
    <section
      className={`rounded-lg border border-gray-200 bg-white p-5 shadow-sm ${className}`}
    >
      {(title || actions) && (
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          {title && <h2 className="text-base font-semibold">{title}</h2>}
          {actions}
        </div>
      )}
      {children}
    </section>
  )
}

export function Field({ label, htmlFor, hint, children }) {
  return (
    <div className="space-y-1">
      <label
        htmlFor={htmlFor}
        className="block text-sm font-medium text-gray-700"
      >
        {label}
      </label>
      {children}
      {hint && <p className="text-xs text-gray-500">{hint}</p>}
    </div>
  )
}

export function ErrorMessage({ error }) {
  if (!error) {
    return null
  }
  return (
    <p className="rounded border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
      {error.message || String(error)}
    </p>
  )
}

const STATUS_STYLES = {
  created: 'bg-gray-100 text-gray-700',
  queued: 'bg-sky-100 text-sky-800',
  running: 'bg-amber-100 text-amber-800',
  paused: 'bg-gray-100 text-gray-700',
  completed: 'bg-emerald-100 text-emerald-800',
  failed: 'bg-rose-100 text-rose-800',
  cancelled: 'bg-gray-200 text-gray-600',
}

export function StatusBadge({ status }) {
  return (
    <span
      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${
        STATUS_STYLES[status] || STATUS_STYLES.created
      }`}
    >
      {status}
    </span>
  )
}

// Shown when the API cannot be reached, which usually means the backend or
// its database is not running.
export function BackendUnavailable({ error }) {
  return (
    <Card>
      <p className="text-sm text-gray-700">
        Could not load data from the backend: {error.message}.
      </p>
      <p className="mt-2 text-sm text-gray-500">
        Tournaments need the backend with a database. Start it with{' '}
        <code className="rounded bg-gray-100 px-1">docker compose up -d</code>.
      </p>
    </Card>
  )
}
