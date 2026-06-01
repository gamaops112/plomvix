import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { ThemeModeToggle } from '../ThemeModeToggle';
import { useAuth } from '../../auth/useAuth';

export function Shell({ children }: { children: ReactNode }) {
  const { user } = useAuth();

  return (
    <div className="shell">
      <Sidebar />
      <div className="shell-content">
        <header className="shell-header">
          <div className="shell-user">
            {user && <span className="shell-username">{user.username}</span>}
          </div>
          <ThemeModeToggle />
          <Link to="/logout" className="shell-logout-link">Logout</Link>
        </header>
        <main className="shell-main">{children}</main>
      </div>
    </div>
  );
}
