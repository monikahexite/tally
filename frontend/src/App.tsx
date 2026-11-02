import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { Layout } from './components/Layout'
import { NotFound, RequireAuth } from './components/common'
import { AuthProvider } from './lib/auth'
import { AuthPage } from './pages/AuthPage'
import { CreatePoll } from './pages/CreatePoll'
import { Dashboard } from './pages/Dashboard'
import { Home } from './pages/Home'
import { PollPage } from './pages/PollPage'
import { PresentPage } from './pages/PresentPage'

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/p/:slug/present" element={<PresentPage />} />
          <Route element={<Layout />}>
            <Route index element={<Home />} />
            <Route path="login" element={<AuthPage mode="login" />} />
            <Route path="signup" element={<AuthPage mode="signup" />} />
            <Route
              path="dashboard"
              element={
                <RequireAuth>
                  <Dashboard />
                </RequireAuth>
              }
            />
            <Route
              path="new"
              element={
                <RequireAuth>
                  <CreatePoll />
                </RequireAuth>
              }
            />
            <Route path="p/:slug" element={<PollPage />} />
            <Route path="*" element={<NotFound />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
