import type { ReactNode } from 'react'
import { Link, Navigate, useLocation } from 'react-router-dom'
import { useAuth } from '../lib/authContext'

export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return (
    <div className="center-state" role="status">
      <span className="spinner" aria-hidden />
      <span className="muted">{label}</span>
    </div>
  )
}

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="center-state">
      <h2>{title}</h2>
      {children}
    </div>
  )
}

export function NotFound() {
  return (
    <EmptyState title="Nothing here">
      <p className="muted">This page or poll doesn't exist, or it was deleted.</p>
      <Link className="btn btn--primary" to="/">
        Go home
      </Link>
    </EmptyState>
  )
}

/** Sends logged-out visitors to /login and brings them back afterwards. */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()
  const location = useLocation()
  if (loading) return <Spinner />
  if (!user) return <Navigate to={`/login?next=${encodeURIComponent(location.pathname)}`} replace />
  return <>{children}</>
}

interface FieldProps {
  label: string
  error?: string
  hint?: string
  children: ReactNode
}

export function Field({ label, error, hint, children }: FieldProps) {
  return (
    <label className={`field ${error ? 'field--error' : ''}`}>
      <span className="field__label">{label}</span>
      {children}
      {error ? <span className="field__error">{error}</span> : hint && <span className="field__hint">{hint}</span>}
    </label>
  )
}
