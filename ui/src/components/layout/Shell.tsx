import { useState } from 'react';
import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { ThemeModeToggle } from '../ThemeModeToggle';
import { Button } from '@/components/ui/button';
import { useAuth } from '../../auth/useAuth';

export function Shell({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div className="grid h-full" style={{ gridTemplateColumns: 'var(--plx-sidebar-width) 1fr' }}>
      <div className={`fixed inset-y-0 left-0 z-40 transform transition-transform bg-card border-r lg:relative lg:translate-x-0 ${sidebarOpen ? 'translate-x-0' : '-translate-x-full'}`}>
        <Sidebar onNavigate={() => setSidebarOpen(false)} />
      </div>
      {sidebarOpen && <div className="fixed inset-0 z-30 bg-black/40 lg:hidden" onClick={() => setSidebarOpen(false)} aria-hidden />}
      <div className="flex flex-col overflow-hidden col-span-full lg:col-span-1 lg:col-start-2">
        <header className="flex items-center justify-between gap-4 px-4 py-2 border-b bg-card h-[var(--plx-navbar-height)]">
          <Button variant="ghost" size="icon-sm" className="lg:hidden" onClick={() => setSidebarOpen(true)} aria-label="Open menu">
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><line x1="3" y1="6" x2="21" y2="6"/><line x1="3" y1="12" x2="21" y2="12"/><line x1="3" y1="18" x2="21" y2="18"/></svg>
          </Button>
          <div className="flex items-center gap-4 ml-auto">
            {user && <span className="text-sm font-medium text-muted-foreground">{user.username}</span>}
            <ThemeModeToggle />
            <Link to="/logout" className="text-sm text-accent hover:underline">Logout</Link>
          </div>
        </header>
        <main className="flex-1 overflow-y-auto p-4 lg:p-8">{children}</main>
      </div>
    </div>
  );
}
