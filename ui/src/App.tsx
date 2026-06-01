import { Routes, Route, Navigate } from 'react-router-dom';
import { Shell } from './components/layout/Shell';
import { AppRoutes } from './app/AppRoutes';
import { LoginPage } from './pages/LoginPage';
import { LogoutPage } from './pages/LogoutPage';
import { NotFoundPage } from './pages/NotFoundPage';
import { ProtectedRoute } from './auth/ProtectedRoute';
import { ToastViewport } from './components/feedback/ToastViewport';
import { useAuth } from './auth/useAuth';

function AppShell() {
  return (
    <Shell>
      <AppRoutes />
    </Shell>
  );
}

function DefaultRedirect() {
  const { authenticated, loading } = useAuth();
  if (loading) return null;
  return <Navigate to={authenticated ? '/app/explore' : '/login'} replace />;
}

export function App() {
  return (
    <>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/logout" element={<LogoutPage />} />
        <Route path="/app/*" element={
          <ProtectedRoute>
            <AppShell />
          </ProtectedRoute>
        } />
        <Route path="/dev/design" element={
          <ProtectedRoute>
            <Shell>
              <AppRoutes />
            </Shell>
          </ProtectedRoute>
        } />
        <Route path="/" element={<DefaultRedirect />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
      <ToastViewport />
    </>
  );
}
