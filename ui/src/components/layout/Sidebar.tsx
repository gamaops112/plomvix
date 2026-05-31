import { NavLink } from 'react-router-dom';
import { navRoutes } from '../../app/routes';

export function Sidebar() {
  return (
    <nav className="sidebar">
      <div className="sidebar-logo">Plomvix</div>
      <ul className="sidebar-nav">
        {navRoutes.map((route) => (
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
