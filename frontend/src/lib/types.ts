export interface User {
  id: string
  name: string
  email: string
}

export interface Option {
  id: string
  text: string
}

export type PollStatus = 'open' | 'closed'

export interface Poll {
  slug: string
  question: string
  description: string
  options: Option[]
  status: PollStatus
  closesAt: string | null
  createdAt: string
  ownerName: string
}

/** A versioned snapshot of vote counts. Higher seq = newer. */
export interface Results {
  seq: number
  counts: Record<string, number>
}

export interface PollView {
  poll: Poll
  results: Results
  myVote: string | null
  isOwner: boolean
}

export interface PollSummary extends Poll {
  totalVotes: number
}

/** Messages pushed by the server over Server-Sent Events. */
export type LiveEvent =
  | ({ type: 'snapshot'; status: PollStatus; closesAt: string | null; viewers: number } & Results)
  | ({ type: 'counts' } & Results)
  | { type: 'viewers'; count: number }
  | { type: 'status'; status: PollStatus; closesAt: string | null }
  | { type: 'deleted' }
