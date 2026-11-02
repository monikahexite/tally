import { useRef, useState, type FormEvent, type KeyboardEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Field } from '../components/common'
import { api, ApiError } from '../lib/api'

const MAX_OPTIONS = 10
const DURATIONS = [
  { label: 'No time limit', minutes: 0 },
  { label: '5 minutes', minutes: 5 },
  { label: '15 minutes', minutes: 15 },
  { label: '1 hour', minutes: 60 },
  { label: '1 day', minutes: 60 * 24 },
  { label: '1 week', minutes: 60 * 24 * 7 },
]

export function CreatePoll() {
  const navigate = useNavigate()
  const [question, setQuestion] = useState('')
  const [description, setDescription] = useState('')
  const [options, setOptions] = useState(['', ''])
  const [duration, setDuration] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [fields, setFields] = useState<Record<string, string>>({})
  const optionRefs = useRef<(HTMLInputElement | null)[]>([])

  function setOption(i: number, value: string) {
    setOptions((prev) => prev.map((o, idx) => (idx === i ? value : o)))
  }

  function addOption(focus = true) {
    if (options.length >= MAX_OPTIONS) return
    setOptions((prev) => [...prev, ''])
    if (focus) requestAnimationFrame(() => optionRefs.current[options.length]?.focus())
  }

  function removeOption(i: number) {
    setOptions((prev) => prev.filter((_, idx) => idx !== i))
  }

  // Enter on the last option adds a new one, which makes typing a list quick.
  function onOptionKey(e: KeyboardEvent<HTMLInputElement>, i: number) {
    if (e.key !== 'Enter') return
    e.preventDefault()
    if (i === options.length - 1) addOption()
    else optionRefs.current[i + 1]?.focus()
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    setFields({})
    try {
      const { poll } = await api.createPoll({
        question,
        description,
        options: options.filter((o) => o.trim() !== ''),
        durationMinutes: duration,
      })
      navigate(`/p/${poll.slug}?created=1`)
    } catch (err) {
      if (err instanceof ApiError) {
        setFields(err.fields)
        setError(err.message)
      } else setError('Something went wrong')
      setBusy(false)
    }
  }

  return (
    <form className="create" onSubmit={submit} noValidate>
      <div className="page-head">
        <div>
          <h1>New poll</h1>
          <p className="muted">Ask one clear question. You can share it as soon as it's created.</p>
        </div>
      </div>

      <div className="card create__card">
        {error && <div className="alert">{error}</div>}

        <Field label="Question" error={fields.question}>
          <input
            className="input--lg"
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="What should we build next?"
            maxLength={200}
            autoFocus
          />
        </Field>

        <Field label="Description (optional)" error={fields.description}>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Any context voters should know"
            rows={2}
            maxLength={500}
          />
        </Field>

        <fieldset className="field">
          <legend className="field__label">Options</legend>
          <div className="options-edit">
            {options.map((o, i) => (
              <div key={i} className="option-edit">
                <span className="option-edit__n">{i + 1}</span>
                <input
                  ref={(el) => {
                    optionRefs.current[i] = el
                  }}
                  value={o}
                  onChange={(e) => setOption(i, e.target.value)}
                  onKeyDown={(e) => onOptionKey(e, i)}
                  placeholder={`Option ${i + 1}`}
                  maxLength={100}
                  aria-label={`Option ${i + 1}`}
                />
                <button
                  type="button"
                  className="icon-btn"
                  onClick={() => removeOption(i)}
                  disabled={options.length <= 2}
                  aria-label={`Remove option ${i + 1}`}
                >
                  ×
                </button>
              </div>
            ))}
          </div>
          {fields.options && <span className="field__error">{fields.options}</span>}
          {options.length < MAX_OPTIONS && (
            <button type="button" className="btn btn--ghost btn--sm" onClick={() => addOption()}>
              + Add option
            </button>
          )}
        </fieldset>

        <Field label="Close automatically" error={fields.durationMinutes}>
          <select value={duration} onChange={(e) => setDuration(Number(e.target.value))}>
            {DURATIONS.map((d) => (
              <option key={d.minutes} value={d.minutes}>
                {d.label}
              </option>
            ))}
          </select>
        </Field>

        <button className="btn btn--primary btn--block btn--lg" disabled={busy}>
          {busy ? 'Creating…' : 'Create poll'}
        </button>
      </div>
    </form>
  )
}
