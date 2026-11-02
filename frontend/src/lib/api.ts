import type { Poll, PollSummary, PollView, Results, User } from './types'

export class ApiError extends Error {
  status: number
  code: string
  fields: Record<string, string>

  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
    super(message)
    this.status = status
    this.code = code
    this.fields = fields
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      method,
      credentials: 'same-origin',
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(0, 'network', 'Cannot reach the server. Check your connection.')
  }
  if (res.status === 204) return undefined as T
  const data = await res.json().catch(() => null)
  if (!res.ok) {
    throw new ApiError(res.status, data?.code ?? 'error', data?.error ?? `Request failed (${res.status})`, data?.fields)
  }
  return data as T
}

export interface NewPoll {
  question: string
  description: string
  options: string[]
  durationMinutes: number
}

export const api = {
  me: () => request<{ user: User | null }>('GET', '/auth/me'),
  signup: (name: string, email: string, password: string) =>
    request<{ user: User }>('POST', '/auth/signup', { name, email, password }),
  login: (email: string, password: string) => request<{ user: User }>('POST', '/auth/login', { email, password }),
  logout: () => request<void>('POST', '/auth/logout'),

  createPoll: (p: NewPoll) => request<{ poll: Poll }>('POST', '/polls', p),
  myPolls: () => request<{ polls: PollSummary[] }>('GET', '/polls/mine'),
  getPoll: (slug: string) => request<PollView>('GET', `/polls/${encodeURIComponent(slug)}`),
  vote: (slug: string, optionId: string) =>
    request<{ results: Results; myVote: string }>('POST', `/polls/${encodeURIComponent(slug)}/vote`, { optionId }),
  setStatus: (slug: string, status: 'open' | 'closed') =>
    request<{ poll: Poll }>('PATCH', `/polls/${encodeURIComponent(slug)}`, { status }),
  deletePoll: (slug: string) => request<void>('DELETE', `/polls/${encodeURIComponent(slug)}`),
}

export function pollUrl(slug: string) {
  return `${window.location.origin}/p/${slug}`
}
