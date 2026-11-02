import { useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import { Field } from '../components/common'
import { ApiError } from '../lib/api'
import { useAuth } from '../lib/authContext'

/** Only allow redirects to our own paths, never to another site. */
function safeNext(next: string | null) {
  return next && next.startsWith('/') && !next.startsWith('//') ? next : '/dashboard'
}

export function AuthPage({ mode }: { mode: 'login' | 'signup' }) {
  const { user, login, signup } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const next = safeNext(params.get('next'))

  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [fields, setFields] = useState<Record<string, string>>({})

  if (user) return <Navigate to={next} replace />

  const isSignup = mode === 'signup'

  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError('')
    setFields({})
    try {
      if (isSignup) await signup(name, email, password)
      else await login(email, password)
      navigate(next, { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        setFields(err.fields)
        setError(Object.keys(err.fields).length ? '' : err.message)
      } else {
        setError('Something went wrong')
      }
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth">
      <form className="card auth__card" onSubmit={submit} noValidate>
        <h1>{isSignup ? 'Create your account' : 'Welcome back'}</h1>
        <p className="muted">
          {isSignup ? 'You need an account to create and manage polls. Voting never needs one.' : 'Log in to manage your polls.'}
        </p>

        {error && <div className="alert">{error}</div>}

        {isSignup && (
          <Field label="Name" error={fields.name}>
            <input value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" maxLength={50} required />
          </Field>
        )}
        <Field label="Email" error={fields.email}>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" required />
        </Field>
        <Field
          label="Password"
          error={fields.password}
          hint={isSignup ? 'At least 8 characters, with a letter and a number' : undefined}
        >
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={isSignup ? 'new-password' : 'current-password'}
            maxLength={72}
            required
          />
        </Field>

        <button className="btn btn--primary btn--block" disabled={busy}>
          {busy ? 'Please wait…' : isSignup ? 'Sign up' : 'Log in'}
        </button>

        <p className="muted auth__switch">
          {isSignup ? 'Already have an account? ' : 'New here? '}
          <Link to={`${isSignup ? '/login' : '/signup'}?next=${encodeURIComponent(next)}`}>
            {isSignup ? 'Log in' : 'Create an account'}
          </Link>
        </p>
      </form>
    </div>
  )
}
