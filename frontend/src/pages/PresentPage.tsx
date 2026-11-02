import { QRCodeSVG } from 'qrcode.react'
import { Link, useParams } from 'react-router-dom'
import { EmptyState, NotFound, Spinner } from '../components/common'
import { Logo } from '../components/Layout'
import { LiveBadge } from '../components/LiveBadge'
import { ResultBars } from '../components/ResultBars'
import { useAnimatedNumber } from '../hooks/useAnimatedNumber'
import { useNow } from '../hooks/useNow'
import { usePollStream } from '../hooks/usePollStream'
import { usePollView } from '../hooks/usePollView'
import { pollUrl } from '../lib/api'
import { formatRemaining } from '../lib/format'
import type { PollView } from '../lib/types'

/** Full-screen view for a projector: big live bars and a QR code to join. */
export function PresentPage() {
  const { slug = '' } = useParams()
  const { view, error } = usePollView(slug)
  if (error?.status === 404) return <NotFound />
  if (error) return <EmptyState title="Couldn't load this poll" />
  if (!view) return <Spinner />
  return <Present key={slug} view={view} />
}

function Present({ view }: { view: PollView }) {
  const { poll } = view
  const live = usePollStream(poll.slug, { results: view.results, status: poll.status, closesAt: poll.closesAt })
  const total = poll.options.reduce((s, o) => s + (live.results.counts[o.id] ?? 0), 0)
  const shownTotal = useAnimatedNumber(total)
  const closesAtMs = live.closesAt ? new Date(live.closesAt).getTime() : null
  const now = useNow(live.status === 'open' && closesAtMs !== null)
  const isOpen = live.status === 'open' && (closesAtMs === null || closesAtMs > now)
  const url = pollUrl(poll.slug)

  function fullscreen() {
    if (document.fullscreenElement) document.exitFullscreen()
    else document.documentElement.requestFullscreen().catch(() => {})
  }

  if (live.deleted) return <EmptyState title="This poll was deleted" />

  return (
    <div className="present">
      <header className="present__top">
        <Logo />
        <LiveBadge connection={live.connection} viewers={live.viewers} />
        <div className="present__tools">
          <button className="btn btn--ghost btn--sm" onClick={fullscreen}>
            Fullscreen
          </button>
          <Link className="btn btn--ghost btn--sm" to={`/p/${poll.slug}`}>
            Exit
          </Link>
        </div>
      </header>

      <div className="present__body">
        <section className="present__main">
          <h1>{poll.question}</h1>
          <ResultBars options={poll.options} results={live.results} size="large" />
        </section>

        <aside className="present__side">
          <div className="present__qr">
            <QRCodeSVG value={url} size={220} marginSize={2} />
          </div>
          <p className="present__join">
            {isOpen ? 'Scan or visit' : 'Voting is closed'}
            {isOpen && <strong>{url.replace(/^https?:\/\//, '')}</strong>}
          </p>
          <div className="present__total">
            <span>{shownTotal.toLocaleString()}</span>
            <small>{total === 1 ? 'vote' : 'votes'}</small>
          </div>
          {isOpen && closesAtMs !== null && <p className="muted">Closes in {formatRemaining(closesAtMs - now)}</p>}
        </aside>
      </div>
    </div>
  )
}
