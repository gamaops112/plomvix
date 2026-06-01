import type { ReactNode } from 'react';
import { HomePlaceholder } from '../pages/HomePlaceholder';
import { ExplorePlaceholder } from '../pages/ExplorePlaceholder';
import { AdminPage } from '../admin/AdminPage';
import { DevDesignPage } from '../pages/dev/DevDesignPage';

export type AppRoute = {
  path: string;
  label: string;
  element: ReactNode;
  nav: boolean;
  devOnly?: boolean;
  adminOnly?: boolean;
  group?: string;
};

export const appRoutes: AppRoute[] = [
  { path: '/app',           label: 'Home',    element: <HomePlaceholder />,    nav: false },
  { path: '/app/explore',   label: 'Explore', element: <ExplorePlaceholder />, nav: true  },
  {
    path: '/app/admin',
    label: 'Admin',
    element: <AdminPage />,
    nav: true,
    adminOnly: true,
  },
  {
    path: '/dev/design',
    label: 'Design Panel',
    element: <DevDesignPage />,
    nav: true,
    devOnly: true,
    group: 'Developer',
  },
];

export const navRoutes = appRoutes.filter((route) => route.nav);
