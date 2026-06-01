import { useState, type FormEvent } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuth } from '../auth/useAuth';
import { safeNextPath } from '../auth/redirect';
import { useAppEvents } from '../events/AppEventProvider';
import { Button } from '@/components/ui/button';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertTitle } from '@/components/ui/alert';

export function LoginPage() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { login } = useAuth();
  const { emit } = useAppEvents();

  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');

    if (!username.trim() || !password.trim()) {
      setError('Username and password are required.');
      return;
    }

    setSubmitting(true);
    try {
      await login(username, password);
      emit({ type: 'toast:add', payload: { kind: 'success', title: 'Welcome back' } });
      navigate(safeNextPath(searchParams.get('next')), { replace: true });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Login failed';
      setError(msg);
      emit({ type: 'toast:add', payload: { kind: 'error', title: 'Login failed', message: msg } });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen p-4 bg-background">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-center text-2xl">Plomvix</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} noValidate>
            <div className="space-y-1 mb-4">
              <Label htmlFor="login-username">Username</Label>
              <Input
                id="login-username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoComplete="username"
                autoFocus
                disabled={submitting}
              />
            </div>
            <div className="space-y-1 mb-4">
              <Label htmlFor="login-password">Password</Label>
              <Input
                id="login-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                disabled={submitting}
              />
            </div>
            {error && (
              <Alert variant="destructive" className="mb-4">
                <AlertTitle>{error}</AlertTitle>
              </Alert>
            )}
            <Button type="submit" variant="default" disabled={submitting} className="w-full">
              {submitting ? 'Signing in...' : 'Sign In'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
