export function percent(part: number, total: number) {
  return total === 0 ? 0 : Math.round((part / total) * 100)
}

export function plural(n: number, word: string) {
  return `${n.toLocaleString()} ${word}${n === 1 ? '' : 's'}`
}

/** "2d 4h", "12m 05s", "45s" */
export function formatRemaining(ms: number) {
  const s = Math.max(0, Math.floor(ms / 1000))
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${String(m).padStart(2, '0')}m`
  if (m > 0) return `${m}m ${String(sec).padStart(2, '0')}s`
  return `${sec}s`
}

export function timeAgo(iso: string) {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return new Date(iso).toLocaleDateString(undefined, { day: 'numeric', month: 'short' })
}

/** Accepts a bare code ("k7m2xq9a") or any URL ending in /p/<code>. */
export function extractSlug(input: string): string | null {
  const trimmed = input.trim().toLowerCase()
  const match = trimmed.match(/(?:\/p\/)?([a-z0-9]{4,16})\/?$/)
  return match ? match[1] : null
}

/** Ten distinguishable hues for options, tuned to read on light and dark. */
export const OPTION_COLORS = [
  '#ff5a36', '#3b82f6', '#10b981', '#f59e0b', '#a855f7',
  '#ec4899', '#14b8a6', '#6366f1', '#84cc16', '#ef4444',
]
