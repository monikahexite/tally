import { useEffect, useRef, useState } from 'react'

/** Smoothly tweens a number towards `target` (used for vote counters). */
export function useAnimatedNumber(target: number, duration = 450) {
  const [value, setValue] = useState(target)
  const from = useRef(target)

  useEffect(() => {
    const start = performance.now()
    const begin = from.current
    if (begin === target) return
    let frame = 0
    const tick = (t: number) => {
      const p = Math.min(1, (t - start) / duration)
      const eased = 1 - Math.pow(1 - p, 3)
      const next = begin + (target - begin) * eased
      from.current = next
      setValue(next)
      if (p < 1) frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(frame)
  }, [target, duration])

  return Math.round(value)
}
