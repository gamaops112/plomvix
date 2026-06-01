import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from './useAuth';

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { authenticated, loading } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '200px' }}>
        <p>Loading...</p>
      </div>
    );
  }

  if (!authenticated) {
    const next = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/login?next=${next}`} replace />;
  }

  return <>{children}</>;
}
