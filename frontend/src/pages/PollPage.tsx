import { useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { EmptyState, NotFound, Spinner } from '../components/common'
import { LiveBadge } from '../components/LiveBadge'
import { ResultBars } from '../components/ResultBars'
import { SharePanel } from '../components/SharePanel'
import { useNow } from '../hooks/useNow'
import { usePollView } from '../hooks/usePollView'
import { usePollStream } from '../hooks/usePollStream'
import { api, ApiError } from '../lib/api'
import { OPTION_COLORS, formatRemaining, plural } from '../lib/format'
import type { PollView } from '../lib/types'

export function PollPage() {
  const { slug = '' } = useParams()
  const { view, error } = usePollView(slug)

  if (error?.status === 404) return <NotFound />
  if (error)
    return (
      <EmptyState title="Couldn't load this poll">
        <p className="muted">{error.message}</p>
      </EmptyState>
    )
  if (!view) return <Spinner />
  return <LivePoll key={slug} view={view} />
}

function LivePoll({ view }: { view: PollView }) {
  const { poll, isOwner } = view
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const live = usePollStream(poll.slug, { results: view.results, status: poll.status, closesAt: poll.closesAt })

  const [myVote, setMyVote] = useState(view.myVote)
  const [selected, setSelected] = useState<string | null>(null)
  const [showResults, setShowResults] = useState(view.myVote !== null || isOwner)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')

  const closesAtMs = live.closesAt ? new Date(live.closesAt).getTime() : null
  const now = useNow(live.status === 'open' && closesAtMs !== null)
  const isOpen = live.status === 'open' && (closesAtMs === null || closesAtMs > now)
  const total = poll.options.reduce((s, o) => s + (live.results.counts[o.id] ?? 0), 0)
  const canVote = isOpen && myVote === null

  async function submitVote() {
    if (!selected) return
    setBusy(true)
    setMessage('')
    try {
      const res = await api.vote(poll.slug, selected)
      setMyVote(res.myVote)
      live.applyResults(res.results)
      setShowResults(true)
    } catch (e) {
      const err = e instanceof ApiError ? e : null
      if (err?.code === 'already_voted') setShowResults(true)
      setMessage(err?.message ?? 'Could not submit your vote')
    } finally {
      setBusy(false)
    }
  }

  async function toggleStatus() {
    try {
      await api.setStatus(poll.slug, isOpen ? 'closed' : 'open')
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : 'Could not update the poll')
    }
  }

  async function remove() {
    if (!window.confirm('Delete this poll and all its votes? This cannot be undone.')) return
    try {
      await api.deletePoll(poll.slug)
      navigate('/dashboard')
    } catch (e) {
      setMessage(e instanceof ApiError ? e.message : 'Could not delete the poll')
    }
  }

  if (live.deleted && !isOwner) {
    return (
      <EmptyState title="This poll was deleted">
        <p className="muted">Its creator removed it.</p>
        <Link className="btn btn--primary" to="/">
          Go home
        </Link>
      </EmptyState>
    )
  }

  return (
    <div className="poll-layout">
      <section className="card poll">
        <div className="poll__meta">
          <LiveBadge connection={live.connection} viewers={live.viewers} />
          {isOpen ? (
            closesAtMs !== null && <span className="chip chip--open">Closes in {formatRemaining(closesAtMs - now)}</span>
          ) : (
            <span className="chip chip--closed">Voting closed</span>
          )}
        </div>

        <h1 className="poll__question">{poll.question}</h1>
        {poll.description && <p className="poll__desc">{poll.description}</p>}
        <p className="muted small">
          by {poll.ownerName} · {plural(total, 'vote')}
        </p>

        {message && <div className="alert">{message}</div>}

        {canVote && !showResults ? (
          <>
            <div className="choices" role="radiogroup" aria-label="Options">
              {poll.options.map((o, i) => (
                <button
                  key={o.id}
                  type="button"
                  role="radio"
                  aria-checked={selected === o.id}
                  className={`choice ${selected === o.id ? 'choice--selected' : ''}`}
                  style={{ '--bar': OPTION_COLORS[i % OPTION_COLORS.length] } as React.CSSProperties}
                  onClick={() => setSelected(o.id)}
                >
                  <span className="choice__radio" aria-hidden />
                  {o.text}
                </button>
              ))}
            </div>
            <div className="poll__actions">
              <button className="btn btn--primary btn--lg" disabled={!selected || busy} onClick={submitVote}>
                {busy ? 'Submitting…' : 'Submit vote'}
              </button>
              <button className="btn btn--link" onClick={() => setShowResults(true)}>
                Just show me the results
              </button>
            </div>
          </>
        ) : (
          <>
            <ResultBars options={poll.options} results={live.results} myVote={myVote} />
            {canVote && (
              <button className="btn btn--link" onClick={() => setShowResults(false)}>
                ← Back to voting
              </button>
            )}
            {myVote && <p className="muted small thanks">Thanks for voting! Results keep updating live.</p>}
          </>
        )}
      </section>

      <aside className="poll-side">
        {isOwner && params.get('created') && (
          <div className="alert alert--success">Your poll is live. Share the link below to start collecting votes.</div>
        )}
        <SharePanel slug={poll.slug} question={poll.question} />
        {isOwner && (
          <section className="card owner">
            <h3>Manage</h3>
            <p className="muted small">Only you can see these controls.</p>
            <div className="owner__actions">
              <button className="btn btn--dark" onClick={toggleStatus}>
                {isOpen ? 'Close voting' : 'Reopen voting'}
              </button>
              <button className="btn btn--danger" onClick={remove}>
                Delete poll
              </button>
            </div>
          </section>
        )}
      </aside>
    </div>
  )
}
