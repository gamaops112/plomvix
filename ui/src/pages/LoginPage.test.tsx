import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import React from 'react';
import { LoginPage } from './LoginPage';
import { AuthContext } from '../auth/AuthContext';
import { AppEventProvider } from '../events/AppEventProvider';
import type { AuthUser } from '../auth/types';

type AuthContextValue = NonNullable<React.ContextType<typeof AuthContext>>;

const mockUser: AuthUser = {
  id: '1',
  username: 'admin',
  role: 'admin',
  created_at: '',
  updated_at: '',
};

function renderLoginPage(mockAuth: Partial<AuthContextValue> = {}) {
  const auth: AuthContextValue = {
    user: null,
    loading: false,
    authenticated: false,
    login: vi.fn() as AuthContextValue['login'],
    logout: vi.fn() as AuthContextValue['logout'],
    refresh: vi.fn() as AuthContextValue['refresh'],
    ...mockAuth,
  };

  return render(
    <MemoryRouter initialEntries={['/login']}>
      <AppEventProvider>
        <AuthContext.Provider value={auth}>
          <LoginPage />
        </AuthContext.Provider>
      </AppEventProvider>
    </MemoryRouter>
  );
}

describe('LoginPage', () => {
  it('renders username and password fields', () => {
    renderLoginPage();
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
  });

  it('shows inline error when submitting empty form', async () => {
    renderLoginPage();
    const button = screen.getByRole('button', { name: /sign in/i });
    await userEvent.click(button);
    expect(screen.getByText(/username and password are required/i)).toBeInTheDocument();
  });

  it('calls login when form is filled', async () => {
    const login = vi.fn().mockResolvedValue(mockUser) as AuthContextValue['login'];
    renderLoginPage({ login });

    await userEvent.type(screen.getByLabelText(/username/i), 'admin');
    await userEvent.type(screen.getByLabelText(/password/i), 'password');
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(login).toHaveBeenCalledWith('admin', 'password');
    });
  });

  it('disables submit button while logging in', async () => {
    let resolveLogin: (value: AuthUser) => void;
    const loginPromise = new Promise<AuthUser>((resolve) => { resolveLogin = resolve; });
    const login = vi.fn().mockReturnValue(loginPromise) as AuthContextValue['login'];

    renderLoginPage({ login });

    await userEvent.type(screen.getByLabelText(/username/i), 'admin');
    await userEvent.type(screen.getByLabelText(/password/i), 'password');

    const button = screen.getByRole('button', { name: /sign in/i });
    await userEvent.click(button);

    expect(screen.getByRole('button', { name: /signing in/i })).toBeDisabled();

    resolveLogin!(mockUser);
  });
});
