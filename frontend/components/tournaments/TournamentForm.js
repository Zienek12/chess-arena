import { useState } from 'react'
import { useRouter } from 'next/router'
import { mutate } from 'swr'
import { Button, ErrorMessage, Field, inputClass } from '../ui'
import { createStartingPositionSet } from './PositionSetForm'
import {
  apiPost,
  engineLabel,
  useEngines,
  usePositionSets,
} from '../../lib/api'

const LIMITS = {
  movetime_ms: { label: 'Time per move (ms)', defaultValue: 1000 },
  depth: { label: 'Depth (plies)', defaultValue: 4 },
  nodes: { label: 'Nodes', defaultValue: 100000 },
}

const COLOR_ORDERS = {
  alternate: 'Alternate (each position played with both colors)',
  engine_a_white: 'Engine A always White',
  engine_b_white: 'Engine B always White',
}

// Creates a tournament between any two registered engine versions.
export default function TournamentForm() {
  const router = useRouter()
  const { data: engines = [] } = useEngines()
  const { data: positionSets = [] } = usePositionSets()
  const [form, setForm] = useState({
    name: '',
    engineA: '',
    engineB: '',
    positionSet: '',
    gameCount: 10,
    limitType: 'movetime_ms',
    limitValue: LIMITS.movetime_ms.defaultValue,
    colorOrder: 'alternate',
    cpus: 1,
    memoryMb: 256,
  })
  const [error, setError] = useState(null)
  const [saving, setSaving] = useState(false)

  // Fall back to the first options until the user picks something.
  const engineA = form.engineA || engines[0]?.id || ''
  const engineB =
    form.engineB || engines.find((engine) => engine.id !== engineA)?.id || ''
  const positionSet = form.positionSet || positionSets[0]?.id || ''

  function update(field, parse = (value) => value) {
    return (event) => setForm({ ...form, [field]: parse(event.target.value) })
  }

  function changeLimitType(event) {
    const limitType = event.target.value
    setForm({ ...form, limitType, limitValue: LIMITS[limitType].defaultValue })
  }

  function defaultName() {
    const label = (id) => {
      const engine = engines.find((candidate) => candidate.id === id)
      return engine ? engineLabel(engine) : '?'
    }
    return `${label(engineA)} vs ${label(engineB)}`
  }

  async function submit(event) {
    event.preventDefault()
    setSaving(true)
    setError(null)
    try {
      let positionSetId = positionSet
      if (!positionSetId) {
        positionSetId = (await createStartingPositionSet()).id
      }
      const tournament = await apiPost('/api/tournaments', {
        name: form.name.trim() || defaultName(),
        engine_a_id: engineA,
        engine_b_id: engineB,
        position_set_id: positionSetId,
        game_count: form.gameCount,
        limit: { [form.limitType]: form.limitValue },
        color_order: form.colorOrder,
        cpus: form.cpus,
        memory_mb: form.memoryMb,
      })
      await mutate('/api/tournaments')
      router.push(`/tournaments/${tournament.id}`)
    } catch (err) {
      setError(err)
      setSaving(false)
    }
  }

  if (engines.length < 2) {
    return (
      <p className="text-sm text-gray-600">
        Register at least two engine versions to create a tournament.
      </p>
    )
  }

  const engineOptions = engines.map((engine) => (
    <option key={engine.id} value={engine.id}>
      {engineLabel(engine)}
    </option>
  ))

  return (
    <form onSubmit={submit} className="space-y-4">
      <Field label="Name" htmlFor="tournament-name">
        <input
          id="tournament-name"
          className={inputClass}
          value={form.name}
          onChange={update('name')}
          placeholder={defaultName()}
        />
      </Field>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="Engine A" htmlFor="engine-a">
          <select
            id="engine-a"
            className={inputClass}
            value={engineA}
            onChange={update('engineA')}
          >
            {engineOptions}
          </select>
        </Field>
        <Field label="Engine B" htmlFor="engine-b">
          <select
            id="engine-b"
            className={inputClass}
            value={engineB}
            onChange={update('engineB')}
          >
            {engineOptions}
          </select>
        </Field>
      </div>
      {engineA === engineB && (
        <p className="text-sm text-rose-600">Choose two different engines.</p>
      )}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field
          label="Starting positions"
          htmlFor="position-set"
          hint={
            positionSets.length === 0
              ? 'The standard starting position will be used.'
              : undefined
          }
        >
          <select
            id="position-set"
            className={inputClass}
            value={positionSet}
            onChange={update('positionSet')}
            disabled={positionSets.length === 0}
          >
            {positionSets.length === 0 && (
              <option value="">Starting position</option>
            )}
            {positionSets.map((set) => (
              <option key={set.id} value={set.id}>
                {set.source_name} ({set.position_count})
              </option>
            ))}
          </select>
        </Field>
        <Field label="Games" htmlFor="game-count">
          <input
            id="game-count"
            type="number"
            min="1"
            max="10000"
            className={inputClass}
            value={form.gameCount}
            onChange={update('gameCount', Number)}
          />
        </Field>
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <Field label="Search limit" htmlFor="limit-type">
          <select
            id="limit-type"
            className={inputClass}
            value={form.limitType}
            onChange={changeLimitType}
          >
            {Object.entries(LIMITS).map(([value, limit]) => (
              <option key={value} value={value}>
                {limit.label}
              </option>
            ))}
          </select>
        </Field>
        <Field
          label="Value"
          htmlFor="limit-value"
          hint={
            form.limitType === 'movetime_ms' && form.limitValue < 1000
              ? 'On Docker Desktop, limits under 1000 ms can cause time forfeits.'
              : undefined
          }
        >
          <input
            id="limit-value"
            type="number"
            min="1"
            className={inputClass}
            value={form.limitValue}
            onChange={update('limitValue', Number)}
          />
        </Field>
      </div>
      <details className="rounded border border-gray-200 p-3">
        <summary className="cursor-pointer text-sm font-medium text-gray-700">
          Advanced
        </summary>
        <div className="mt-3 space-y-3">
          <Field label="Colors" htmlFor="color-order">
            <select
              id="color-order"
              className={inputClass}
              value={form.colorOrder}
              onChange={update('colorOrder')}
            >
              {Object.entries(COLOR_ORDERS).map(([value, label]) => (
                <option key={value} value={value}>
                  {label}
                </option>
              ))}
            </select>
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="CPUs per engine" htmlFor="cpus">
              <input
                id="cpus"
                type="number"
                min="0.1"
                step="0.1"
                className={inputClass}
                value={form.cpus}
                onChange={update('cpus', Number)}
              />
            </Field>
            <Field label="Memory per engine (MB)" htmlFor="memory">
              <input
                id="memory"
                type="number"
                min="16"
                className={inputClass}
                value={form.memoryMb}
                onChange={update('memoryMb', Number)}
              />
            </Field>
          </div>
        </div>
      </details>
      <ErrorMessage error={error} />
      <Button type="submit" disabled={saving || engineA === engineB}>
        {saving ? 'Starting...' : 'Start tournament'}
      </Button>
    </form>
  )
}
