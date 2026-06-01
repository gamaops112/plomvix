import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import React from 'react';
import { ProtectedRoute } from './ProtectedRoute';
import { AuthContext } from './AuthContext';
import type { AuthUser } from './types';

type AuthContextValue = NonNullable<React.ContextType<typeof AuthContext>>;

const mockUser: AuthUser = {
  id: '1',
  username: 'admin',
  role: 'admin',
  created_at: '',
  updated_at: '',
};

function renderProtectedRoute(authOverrides: Partial<AuthContextValue>) {
  const auth: AuthContextValue = {
    user: null,
    loading: false,
    authenticated: false,
    login: vi.fn() as AuthContextValue['login'],
    logout: vi.fn() as AuthContextValue['logout'],
    refresh: vi.fn() as AuthContextValue['refresh'],
    ...authOverrides,
  };

  return render(
    <MemoryRouter initialEntries={['/app/dashboard']}>
      <AuthContext.Provider value={auth}>
        <ProtectedRoute>
          <div>Protected Content</div>
        </ProtectedRoute>
      </AuthContext.Provider>
    </MemoryRouter>
  );
}

describe('ProtectedRoute', () => {
  it('shows loading state while auth is loading', () => {
    renderProtectedRoute({ loading: true });
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('renders children when authenticated', () => {
    renderProtectedRoute({
      authenticated: true,
      user: mockUser,
    });
    expect(screen.getByText('Protected Content')).toBeInTheDocument();
  });

  it('redirects unauthenticated user to /login', () => {
    renderProtectedRoute({ authenticated: false });
    expect(screen.queryByText('Protected Content')).not.toBeInTheDocument();
  });
});
