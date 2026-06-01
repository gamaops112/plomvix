import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/useAuth';

export function LogoutPage() {
  const navigate = useNavigate();
  const { logout } = useAuth();
  const [signingOut, setSigningOut] = useState(true);

  useEffect(() => {
    let cancelled = false;
    logout().finally(() => {
      if (!cancelled) {
        setSigningOut(false);
        navigate('/login', { replace: true });
      }
    });
    return () => { cancelled = true; };
  }, [logout, navigate]);

  if (!signingOut) return null;

  return (
    <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '200px' }}>
      <p>Signing out…</p>
    </div>
  );
}
