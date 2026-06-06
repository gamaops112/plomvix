import { useEffect } from 'react'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { useNavigate } from 'react-router-dom'
import AppShell from './layout/AppShell'
import AuthGuard from './components/guards/AuthGuard'
import LoginPage from './pages/auth/LoginPage'
import ForgotPasswordPage from './pages/auth/ForgotPasswordPage'
import Dashboard from './pages/dashboard'
import Logs from './pages/logs'
import Traces from './pages/traces'
import TraceDetailPage from './pages/traces/TraceDetailPage'
import Metrics from './pages/metrics'
import HostDetailPage from './pages/metrics/HostDetailPage'
import APM from './pages/apm'
import Synthetics from './pages/synthetics'
import Alerts from './pages/alerts'
import AlertDetailPage from './pages/alerts/components/AlertDetailPage'
import Incidents from './pages/incidents'
import IncidentDetailPage from './pages/incidents/IncidentDetailPage'
import Users from './pages/users'
import ProfilePage from './pages/profile'
import Integrations from './pages/integrations'
import Settings from './pages/settings'
import Demo from './pages/demo'
import DocsPage from './pages/docs'
import NotFoundPage from './pages/NotFoundPage'
import { setSessionExpiredHandler } from './api/client'
import { useAuthStore } from './store/authStore'

function SessionWatcher() {
  const navigate = useNavigate()
  const logout = useAuthStore((s) => s.logout)
  useEffect(() => {
    setSessionExpiredHandler(() => {
      logout()
      navigate('/login', { replace: true })
    })
  }, [logout, navigate])
  return null
}

export default function App() {
  return (
    <BrowserRouter basename="/app">
      <SessionWatcher />
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/*" element={
          <AuthGuard>
            <AppShell>
              <Routes>
                <Route path="/" element={<Dashboard />} />
                <Route path="/logs" element={<Logs />} />
                <Route path="/traces" element={<Traces />} />
                <Route path="/traces/:id" element={<TraceDetailPage />} />
                <Route path="/metrics" element={<Metrics />} />
                <Route path="/metrics/hosts/:id" element={<HostDetailPage />} />
                <Route path="/apm" element={<APM />} />
                <Route path="/synthetics" element={<Synthetics />} />
                <Route path="/alerts" element={<Alerts />} />
                <Route path="/alerts/:id" element={<AlertDetailPage />} />
                <Route path="/incidents" element={<Incidents />} />
                <Route path="/incidents/:id" element={<IncidentDetailPage />} />
                <Route path="/users" element={<Users />} />
                <Route path="/profile" element={<ProfilePage />} />
                <Route path="/integrations" element={<Integrations />} />
                <Route path="/settings" element={<Settings />} />
                <Route path="/demo" element={<Demo />} />
                <Route path="/docs" element={<DocsPage />} />
                <Route path="*" element={<NotFoundPage />} />
              </Routes>
            </AppShell>
          </AuthGuard>
        } />
      </Routes>
    </BrowserRouter>
  )
}
