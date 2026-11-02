import { useAnimatedNumber } from '../hooks/useAnimatedNumber'
import { OPTION_COLORS, percent } from '../lib/format'
import type { Option, Results } from '../lib/types'

interface Props {
  options: Option[]
  results: Results
  myVote?: string | null
  size?: 'normal' | 'large'
}

export function ResultBars({ options, results, myVote, size = 'normal' }: Props) {
  const total = options.reduce((sum, o) => sum + (results.counts[o.id] ?? 0), 0)
  const max = Math.max(0, ...options.map((o) => results.counts[o.id] ?? 0))

  return (
    <ul className={`results results--${size}`} aria-live="polite">
      {options.map((o, i) => {
        const count = results.counts[o.id] ?? 0
        return (
          <ResultRow
            key={o.id}
            text={o.text}
            count={count}
            pct={percent(count, total)}
            color={OPTION_COLORS[i % OPTION_COLORS.length]}
            leading={total > 0 && count === max}
            mine={myVote === o.id}
          />
        )
      })}
    </ul>
  )
}

interface RowProps {
  text: string
  count: number
  pct: number
  color: string
  leading: boolean
  mine: boolean
}

function ResultRow({ text, count, pct, color, leading, mine }: RowProps) {
  const shown = useAnimatedNumber(count)
  return (
    <li className={`result ${leading ? 'result--leading' : ''}`} style={{ '--bar': color } as React.CSSProperties}>
      <div className="result__label">
        <span className="result__text">
          {text}
          {mine && <span className="chip chip--mine">Your vote</span>}
        </span>
        <span className="result__nums">
          <strong>{pct}%</strong>
          <span className="muted">{shown.toLocaleString()}</span>
        </span>
      </div>
      <div className="result__track">
        <div className="result__fill" style={{ width: `${pct}%` }} />
      </div>
    </li>
  )
}
