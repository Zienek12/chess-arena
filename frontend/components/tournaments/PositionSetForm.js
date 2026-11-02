import { useState } from 'react'
import { mutate } from 'swr'
import { DEFAULT_POSITION } from 'chess.js'
import { Button, ErrorMessage, Field, inputClass } from '../ui'
import { apiPost } from '../../lib/api'
import { OPENINGS, OPENING_SUITE_NAME, openingsPGN } from '../../lib/openings'

const PLACEHOLDERS = {
  fen: `# One FEN per line, optionally named after a semicolon\n${DEFAULT_POSITION} ; Starting position`,
  pgn: '[Event "Ruy Lopez"]\n\n1. e4 e5 2. Nf3 Nc6 3. Bb5 *\n\n[Event "Queen\'s Gambit"]\n\n1. d4 d5 2. c4 *',
}

export async function createStartingPositionSet() {
  const set = await apiPost('/api/position-sets', {
    name: 'Starting position',
    format: 'fen',
    content: `${DEFAULT_POSITION} ; Starting position`,
  })
  await mutate('/api/position-sets')
  return set
}

// Importing the suite again returns the set that already exists.
export async function createOpeningSuite() {
  const set = await apiPost('/api/position-sets', {
    name: OPENING_SUITE_NAME,
    format: 'pgn',
    content: openingsPGN(),
  })
  await mutate('/api/position-sets')
  return set
}

// Imports starting positions from FEN lines or PGN games (the position after
// each game's moves is used, so opening lines work).
export default function PositionSetForm({ onCreated }) {
  const [name, setName] = useState('')
  const [format, setFormat] = useState('pgn')
  const [content, setContent] = useState('')
  const [error, setError] = useState(null)
  const [saving, setSaving] = useState(false)

  async function importSuite() {
    setSaving(true)
    setError(null)
    try {
      onCreated?.(await createOpeningSuite())
    } catch (err) {
      setError(err)
    } finally {
      setSaving(false)
    }
  }

  async function submit(event) {
    event.preventDefault()
    setSaving(true)
    setError(null)
    try {
      const set = await apiPost('/api/position-sets', { name, format, content })
      await mutate('/api/position-sets')
      setName('')
      setContent('')
      onCreated?.(set)
    } catch (err) {
      setError(err)
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3">
      <div className="space-y-2 rounded border border-gray-200 p-3">
        <p className="text-sm text-gray-600">
          {OPENINGS.length} balanced positions from popular openings. Use{' '}
          {OPENINGS.length * 2} games with alternating colors to play each one
          with both colors.
        </p>
        <Button
          type="button"
          variant="secondary"
          disabled={saving}
          onClick={importSuite}
        >
          Add opening suite
        </Button>
      </div>
      <div className="grid grid-cols-3 gap-3">
        <div className="col-span-2">
          <Field label="Name" htmlFor="positions-name">
            <input
              id="positions-name"
              className={inputClass}
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Openings"
            />
          </Field>
        </div>
        <Field label="Format" htmlFor="positions-format">
          <select
            id="positions-format"
            className={inputClass}
            value={format}
            onChange={(event) => setFormat(event.target.value)}
          >
            <option value="pgn">PGN</option>
            <option value="fen">FEN</option>
          </select>
        </Field>
      </div>
      <Field label="Positions" htmlFor="positions-content">
        <textarea
          id="positions-content"
          className={`${inputClass} h-32 font-mono text-xs`}
          required
          value={content}
          onChange={(event) => setContent(event.target.value)}
          placeholder={PLACEHOLDERS[format]}
        />
      </Field>
      <ErrorMessage error={error} />
      <Button type="submit" variant="secondary" disabled={saving}>
        {saving ? 'Importing...' : 'Import positions'}
      </Button>
    </form>
  )
}
