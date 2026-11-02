import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/authContext'
import { extractSlug } from '../lib/format'

export function Home() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [code, setCode] = useState('')
  const [error, setError] = useState('')

  function join(e: FormEvent) {
    e.preventDefault()
    const slug = extractSlug(code)
    if (!slug) {
      setError('Paste a poll link or its 8-character code')
      return
    }
    navigate(`/p/${slug}`)
  }

  return (
    <div className="home">
      <section className="hero">
        <span className="eyebrow">
          <span className="dot dot--live" aria-hidden /> Results update the moment a vote lands
        </span>
        <h1>
          Ask the room.
          <br />
          <span className="accent-text">Watch it answer live.</span>
        </h1>
        <p className="lead">
          Create a poll in seconds, share a link or QR code, and see every vote arrive in real time, on every screen,
          with no refreshing.
        </p>
        <div className="hero__cta">
          <Link to={user ? '/new' : '/signup'} className="btn btn--primary btn--lg">
            Create a poll
          </Link>
          {user && (
            <Link to="/dashboard" className="btn btn--ghost btn--lg">
              My polls
            </Link>
          )}
        </div>

        <form className="join" onSubmit={join}>
          <label htmlFor="join-code" className="muted">
            Got a poll code?
          </label>
          <div className="copy-field">
            <input
              id="join-code"
              placeholder="e.g. k7m2xq9a or a full link"
              value={code}
              onChange={(e) => {
                setCode(e.target.value)
                setError('')
              }}
            />
            <button className="btn btn--dark">Join</button>
          </div>
          {error && <span className="field__error">{error}</span>}
        </form>
      </section>

      <section className="features">
        <Feature title="Live, for everyone" icon="⚡">
          Votes are counted in Redis and pushed over a live stream to every open screen within milliseconds.
        </Feature>
        <Feature title="Share anywhere" icon="🔗">
          A short link plus a QR code. Voters don't need an account; one vote per person is enforced on the server.
        </Feature>
        <Feature title="Built for the room" icon="📽️">
          Present mode puts big live bars and a scannable QR code on the projector while the audience votes.
        </Feature>
      </section>
    </div>
  )
}

function Feature({ title, icon, children }: { title: string; icon: string; children: React.ReactNode }) {
  return (
    <article className="card feature">
      <span className="feature__icon" aria-hidden>
        {icon}
      </span>
      <h3>{title}</h3>
      <p className="muted">{children}</p>
    </article>
  )
}
