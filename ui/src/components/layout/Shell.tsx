import type { ReactNode } from 'react';
import { Sidebar } from './Sidebar';
import { ThemeModeToggle } from '../ThemeModeToggle';

export function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="shell">
      <Sidebar />
      <div className="shell-content">
        <header className="shell-header">
          <ThemeModeToggle />
        </header>
        <main className="shell-main">{children}</main>
      </div>
    </div>
  );
}
