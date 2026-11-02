import type { Connection } from '../hooks/usePollStream'
import { plural } from '../lib/format'

const LABEL: Record<Connection, string> = {
  connecting: 'Connecting…',
  live: 'Live',
  reconnecting: 'Reconnecting…',
  offline: 'Offline',
}

export function LiveBadge({ connection, viewers }: { connection: Connection; viewers: number }) {
  return (
    <div className="live-badge" role="status">
      <span className={`dot dot--${connection}`} aria-hidden />
      <span>{LABEL[connection]}</span>
      {connection === 'live' && viewers > 0 && (
        <span className="muted">· {plural(viewers, 'viewer')}</span>
      )}
    </div>
  )
}
