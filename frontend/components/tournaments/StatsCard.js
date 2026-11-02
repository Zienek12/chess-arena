import { Card } from '../ui'

function signed(value, digits = 1) {
  return `${value > 0 ? '+' : ''}${value.toFixed(digits)}`
}

function Stat({ label, value, hint }) {
  return (
    <div title={hint}>
      <p className="text-xs uppercase tracking-wide text-gray-500">{label}</p>
      <p className="text-xl font-semibold">{value}</p>
    </div>
  )
}

const SPRT_RESULTS = {
  H1: {
    text: 'Passed: H1 accepted',
    className: 'bg-emerald-100 text-emerald-800',
  },
  H0: { text: 'Failed: H0 accepted', className: 'bg-rose-100 text-rose-800' },
}

// Position of the log-likelihood ratio between the SPRT bounds.
function LLRBar({ status }) {
  const { llr, lower_bound: lower, upper_bound: upper } = status
  const position = Math.max(0, Math.min(1, (llr - lower) / (upper - lower)))
  return (
    <div>
      <div className="relative h-2 rounded-full bg-gradient-to-r from-rose-200 via-gray-100 to-emerald-200">
        <div
          className="absolute -top-1 h-4 w-1 rounded bg-gray-900"
          style={{ left: `calc(${position * 100}% - 2px)` }}
        />
      </div>
      <div className="mt-1 flex justify-between font-mono text-xs text-gray-500">
        <span>{lower.toFixed(2)}</span>
        <span className="font-semibold text-gray-900">
          LLR {signed(llr, 2)}
        </span>
        <span>{upper.toFixed(2)}</span>
      </div>
    </div>
  )
}

// Elo difference, its 95% error, the likelihood of superiority and the SPRT,
// all from engine A's point of view.
export default function StatsCard({ tournament, engineA, engineB }) {
  const { stats, sprt_status: sprt } = tournament
  let elo = '–'
  if (stats.elo !== undefined) {
    elo =
      stats.elo_error !== undefined
        ? `${signed(stats.elo)} ± ${stats.elo_error.toFixed(1)}`
        : signed(stats.elo)
  } else if (stats.games > 0) {
    elo = stats.score === 1 ? '+∞' : '−∞'
  }
  const result = sprt && SPRT_RESULTS[sprt.result]

  return (
    <Card title="Statistics" className="mt-6">
      <p className="mb-4 text-sm text-gray-600">
        <span className="font-medium text-gray-900">{engineA}</span> against{' '}
        <span className="font-medium text-gray-900">{engineB}</span>
      </p>
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Stat
          label="Elo difference"
          value={elo}
          hint="Logistic Elo with half the 95% confidence interval; infinite while every game was won or lost"
        />
        <Stat
          label="LOS"
          value={stats.games > 0 ? `${(stats.los * 100).toFixed(1)}%` : '–'}
          hint="Likelihood of superiority: the probability that engine A is stronger"
        />
        <Stat
          label="Score"
          value={stats.games > 0 ? `${(stats.score * 100).toFixed(1)}%` : '–'}
        />
        <Stat
          label="W / D / L"
          value={`${stats.wins} / ${stats.draws} / ${stats.losses}`}
        />
      </div>
      {sprt && (
        <div className="mt-5 border-t border-gray-100 pt-4">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm text-gray-600">
              SPRT: H0 Elo ≤ {sprt.elo0}, H1 Elo ≥ {sprt.elo1} · α {sprt.alpha}{' '}
              · β {sprt.beta}
            </p>
            <span
              className={`rounded px-2 py-0.5 text-xs font-medium ${
                result ? result.className : 'bg-amber-100 text-amber-800'
              }`}
            >
              {result ? result.text : 'Running'}
            </span>
          </div>
          <LLRBar status={sprt} />
          <p className="mt-2 text-xs text-gray-500">
            The tournament stops as soon as the LLR crosses a bound: above the
            upper one engine A is stronger by about {sprt.elo1} Elo or more,
            below the lower one it is not stronger than {sprt.elo0} Elo.
          </p>
        </div>
      )}
    </Card>
  )
}
