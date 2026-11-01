import { useState } from 'react'
import { mutate } from 'swr'
import { Button, ErrorMessage, Field, inputClass } from '../ui'
import { apiPost } from '../../lib/api'

const EMPTY = {
  name: '',
  version: '',
  image: 'chess-arena-engine:local',
  options: '',
}

// Parses "Hash=16, Threads=1" into UCI options.
function parseOptions(text) {
  const options = {}
  for (const entry of text.split(',')) {
    const [name, ...rest] = entry.split('=')
    if (name.trim() && rest.length > 0) {
      options[name.trim()] = rest.join('=').trim()
    }
  }
  return options
}

// Registers an engine version: a name, version and the Docker image that runs it.
export default function EngineForm({ onCreated }) {
  const [form, setForm] = useState(EMPTY)
  const [error, setError] = useState(null)
  const [saving, setSaving] = useState(false)

  function update(field) {
    return (event) => setForm({ ...form, [field]: event.target.value })
  }

  async function submit(event) {
    event.preventDefault()
    setSaving(true)
    setError(null)
    try {
      const engine = await apiPost('/api/engines', {
        name: form.name,
        version: form.version,
        image: form.image,
        uci_options: parseOptions(form.options),
      })
      await mutate('/api/engines')
      setForm(EMPTY)
      onCreated?.(engine)
    } catch (err) {
      setError(err)
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} className="space-y-3">
      <div className="grid grid-cols-2 gap-3">
        <Field label="Name" htmlFor="engine-name">
          <input
            id="engine-name"
            className={inputClass}
            required
            value={form.name}
            onChange={update('name')}
            placeholder="arena"
          />
        </Field>
        <Field label="Version" htmlFor="engine-version-input">
          <input
            id="engine-version-input"
            className={inputClass}
            required
            value={form.version}
            onChange={update('version')}
            placeholder="v2"
          />
        </Field>
      </div>
      <Field
        label="Docker image"
        htmlFor="engine-image"
        hint="Pulled automatically when it is not available locally."
      >
        <input
          id="engine-image"
          className={inputClass}
          required
          value={form.image}
          onChange={update('image')}
        />
      </Field>
      <Field
        label="UCI options"
        htmlFor="engine-options"
        hint="Optional, for example: Hash=16, Threads=1"
      >
        <input
          id="engine-options"
          className={inputClass}
          value={form.options}
          onChange={update('options')}
        />
      </Field>
      <ErrorMessage error={error} />
      <Button type="submit" variant="secondary" disabled={saving}>
        {saving ? 'Adding...' : 'Add engine'}
      </Button>
    </form>
  )
}
