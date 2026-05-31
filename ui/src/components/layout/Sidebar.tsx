import { NavLink } from 'react-router-dom';
import { navRoutes } from '../../app/routes';
import { useTheme } from '../../theme/ThemeContext';

export function Sidebar() {
  const { theme } = useTheme();

  const visible = navRoutes.filter((route) => {
    if (route.devOnly && !theme.dev_panel) {
      return false;
    }
    return true;
  });

  return (
    <nav className="sidebar">
      <div className="sidebar-logo">Plomvix</div>
      <ul className="sidebar-nav">
        {visible.map((route) => (
          <li key={route.path}>
            <NavLink
              to={route.path}
              className={({ isActive }) =>
                `sidebar-nav-item${isActive ? ' sidebar-nav-item--active' : ''}`
              }
            >
              {route.label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}
