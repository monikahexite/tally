import { useEffect, useState } from 'react'
import { api, ApiError } from '../lib/api'
import type { PollView } from '../lib/types'

interface Loaded {
  slug: string
  view: PollView | null
  error: ApiError | null
}

/** Fetches a poll once; everything after that arrives over the live stream. */
export function usePollView(slug: string) {
  const [state, setState] = useState<Loaded | null>(null)

  useEffect(() => {
    let cancelled = false
    api
      .getPoll(slug)
      .then((view) => !cancelled && setState({ slug, view, error: null }))
      .catch(
        (e) =>
          !cancelled &&
          setState({ slug, view: null, error: e instanceof ApiError ? e : new ApiError(0, 'error', 'Could not load poll') }),
      )
    return () => {
      cancelled = true
    }
  }, [slug])

  // Results for a previous slug count as "still loading".
  const current = state?.slug === slug ? state : null
  return { view: current?.view ?? null, error: current?.error ?? null }
}
