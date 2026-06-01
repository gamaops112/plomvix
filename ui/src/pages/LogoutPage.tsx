import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/useAuth';
import { Card, CardContent } from '@/components/ui/card';

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
    <div className="flex items-center justify-center min-h-screen">
      <Card>
        <CardContent className="p-8 text-center">
          <p className="text-muted-foreground">Signing out...</p>
        </CardContent>
      </Card>
    </div>
  );
}
