import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { ThemeModeToggle } from '../ThemeModeToggle';
import { useAuth } from '../../auth/useAuth';

export function Shell({ children }: { children: ReactNode }) {
  const { user } = useAuth();

  return (
    <div className="grid h-full" style={{ gridTemplateColumns: 'var(--plx-sidebar-width) 1fr' }}>
      <Sidebar />
      <div className="flex flex-col overflow-hidden">
        <header className="flex items-center justify-end gap-4 px-4 py-2 border-b bg-card h-[var(--plx-navbar-height)]">
          <div className="shell-user">
            {user && <span className="text-sm font-medium text-muted-foreground">{user.username}</span>}
          </div>
          <ThemeModeToggle />
          <Link to="/logout" className="text-sm text-accent hover:underline">Logout</Link>
        </header>
        <main className="flex-1 overflow-y-auto p-8">{children}</main>
      </div>
    </div>
  );
}
