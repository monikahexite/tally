import { useCallback, useEffect, useReducer } from 'react'
import type { LiveEvent, PollStatus, Results } from '../lib/types'

export type Connection = 'connecting' | 'live' | 'reconnecting' | 'offline'

export interface LiveState {
  results: Results
  status: PollStatus
  closesAt: string | null
  viewers: number
  deleted: boolean
  connection: Connection
}

type Action =
  | { kind: 'event'; event: LiveEvent }
  | { kind: 'connection'; connection: Connection }
  | { kind: 'results'; results: Results }

/** Accept a counts update only if it is newer than what we already show. */
function newer(current: Results, next: Results): Results {
  return next.seq > current.seq ? next : current
}

function reducer(state: LiveState, action: Action): LiveState {
  switch (action.kind) {
    case 'connection':
      return { ...state, connection: action.connection }
    case 'results':
      return { ...state, results: newer(state.results, action.results) }
    case 'event': {
      const ev = action.event
      switch (ev.type) {
        case 'snapshot':
          // A (re)connect snapshot is authoritative: it may legitimately carry
          // a lower seq if the server rebuilt its live state from MongoDB.
          return {
            ...state,
            results: { seq: ev.seq, counts: ev.counts },
            status: ev.status,
            closesAt: ev.closesAt,
            viewers: ev.viewers,
            connection: 'live',
          }
        case 'counts':
          return { ...state, results: newer(state.results, ev) }
        case 'viewers':
          return { ...state, viewers: ev.count }
        case 'status':
          return { ...state, status: ev.status, closesAt: ev.closesAt }
        case 'deleted':
          return { ...state, deleted: true, connection: 'offline' }
      }
    }
  }
  return state
}

/**
 * Subscribes to a poll's live channel. EventSource reconnects on its own after
 * network blips or server restarts; each reconnect starts with a fresh snapshot.
 */
export function usePollStream(slug: string, initial: Pick<LiveState, 'results' | 'status' | 'closesAt'>) {
  const [state, dispatch] = useReducer(reducer, {
    ...initial,
    viewers: 0,
    deleted: false,
    connection: 'connecting',
  })

  useEffect(() => {
    const es = new EventSource(`/api/polls/${encodeURIComponent(slug)}/stream`)
    es.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data) as LiveEvent
        dispatch({ kind: 'event', event })
        if (event.type === 'deleted') es.close()
      } catch {
        /* ignore malformed frames */
      }
    }
    es.onerror = () => {
      dispatch({ kind: 'connection', connection: es.readyState === EventSource.CLOSED ? 'offline' : 'reconnecting' })
    }
    return () => es.close()
  }, [slug])

  /** Merge results we got from our own vote response (may beat the stream). */
  const applyResults = useCallback((results: Results) => dispatch({ kind: 'results', results }), [])

  return { ...state, applyResults }
}
