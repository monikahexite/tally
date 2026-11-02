import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { EmptyState, Spinner } from '../components/common'
import { api, ApiError, pollUrl } from '../lib/api'
import { plural, timeAgo } from '../lib/format'
import type { PollSummary } from '../lib/types'

export function Dashboard() {
  const [polls, setPolls] = useState<PollSummary[] | null>(null)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState('')

  useEffect(() => {
    api
      .myPolls()
      .then((r) => setPolls(r.polls))
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Could not load polls'))
  }, [])

  async function copy(slug: string) {
    await navigator.clipboard.writeText(pollUrl(slug)).catch(() => {})
    setCopied(slug)
    window.setTimeout(() => setCopied(''), 1500)
  }

  if (error) return <EmptyState title="Couldn't load your polls">{<p className="muted">{error}</p>}</EmptyState>
  if (!polls) return <Spinner />

  return (
    <div className="dashboard">
      <div className="page-head">
        <div>
          <h1>My polls</h1>
          <p className="muted">{plural(polls.length, 'poll')}</p>
        </div>
        <Link to="/new" className="btn btn--primary">
          + New poll
        </Link>
      </div>

      {polls.length === 0 ? (
        <EmptyState title="No polls yet">
          <p className="muted">Create your first poll and share the link. Results show up here live.</p>
          <Link to="/new" className="btn btn--primary">
            Create a poll
          </Link>
        </EmptyState>
      ) : (
        <ul className="poll-list">
          {polls.map((p) => (
            <li key={p.slug} className="card poll-item">
              <Link to={`/p/${p.slug}`} className="poll-item__main">
                <span className={`chip chip--${p.status}`}>{p.status === 'open' ? 'Open' : 'Closed'}</span>
                <h3>{p.question}</h3>
                <span className="muted">
                  {plural(p.totalVotes, 'vote')} · {p.options.length} options · created {timeAgo(p.createdAt)}
                </span>
              </Link>
              <div className="poll-item__actions">
                <button className="btn btn--ghost btn--sm" onClick={() => copy(p.slug)}>
                  {copied === p.slug ? 'Copied ✓' : 'Copy link'}
                </button>
                <Link className="btn btn--ghost btn--sm" to={`/p/${p.slug}/present`}>
                  Present
                </Link>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
