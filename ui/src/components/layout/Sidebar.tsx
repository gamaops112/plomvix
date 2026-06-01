import { NavLink } from 'react-router-dom';
import { navRoutes } from '../../app/routes';
import { useTheme } from '../../theme/ThemeContext';
import { useAuth } from '../../auth/useAuth';

export function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  const { theme } = useTheme();
  const { user } = useAuth();

  const visible = navRoutes.filter((route) => {
    if (route.devOnly && !theme.dev_panel) {
      return false;
    }
    if (route.adminOnly && user?.role !== 'admin') {
      return false;
    }
    return true;
  });

  return (
    <nav className="w-[var(--plx-sidebar-width)] bg-card border-r flex flex-col p-4 h-full">
      <div className="text-xl font-bold pb-6 px-2 text-primary">Plomvix</div>
      <ul className="list-none flex flex-col gap-1">
        {visible.map((route) => (
          <li key={route.path}>
            <NavLink
              to={route.path}
              onClick={onNavigate}
              className={({ isActive }) =>
                isActive
                  ? 'block px-3 py-2 rounded-md text-sm bg-primary text-primary-foreground aria-[current]:bg-primary aria-[current]:text-primary-foreground'
                  : 'block px-3 py-2 rounded-md text-sm text-muted-foreground hover:bg-muted hover:text-foreground'
              }
              aria-current={undefined}
            >
              {route.label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}
