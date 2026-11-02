import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/authContext'

export function Logo() {
  return (
    <Link to="/" className="logo" aria-label="Tally home">
      <svg viewBox="0 0 32 32" width="28" height="28" aria-hidden>
        <rect width="32" height="32" rx="8" fill="var(--accent)" />
        <rect x="7" y="17" width="4" height="8" rx="2" fill="#fff" />
        <rect x="14" y="7" width="4" height="18" rx="2" fill="#fff" />
        <rect x="21" y="12" width="4" height="13" rx="2" fill="#fff" />
      </svg>
      <span>Tally</span>
    </Link>
  )
}

export function Layout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  async function handleLogout() {
    await logout()
    navigate('/')
  }

  return (
    <div className="app">
      <header className="topbar">
        <div className="container topbar__inner">
          <Logo />
          <nav className="nav">
            {user ? (
              <>
                <NavLink to="/dashboard" className="nav__link">
                  My polls
                </NavLink>
                <Link to="/new" className="btn btn--primary btn--sm">
                  New poll
                </Link>
                <button className="nav__link nav__btn" onClick={handleLogout} title={user.email}>
                  Log out
                </button>
              </>
            ) : (
              <>
                <NavLink to="/login" className="nav__link">
                  Log in
                </NavLink>
                <Link to="/signup" className="btn btn--primary btn--sm">
                  Sign up
                </Link>
              </>
            )}
          </nav>
        </div>
      </header>
      <main className="container main">
        <Outlet />
      </main>
      <footer className="footer container muted">Tally · React + Go + MongoDB + Redis</footer>
    </div>
  )
}
