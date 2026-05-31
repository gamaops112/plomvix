import type { ReactNode } from 'react';
import { HomePlaceholder } from '../pages/HomePlaceholder';
import { ExplorePlaceholder } from '../pages/ExplorePlaceholder';
import { AdminPlaceholder } from '../pages/AdminPlaceholder';

export type AppRoute = {
  path: string;
  label: string;
  element: ReactNode;
  nav: boolean;
};

export const appRoutes: AppRoute[] = [
  { path: '/',        label: 'Home',    element: <HomePlaceholder />,    nav: false },
  { path: '/explore', label: 'Explore', element: <ExplorePlaceholder />, nav: true  },
  { path: '/admin',   label: 'Admin',   element: <AdminPlaceholder />,   nav: true  },
];

export const navRoutes = appRoutes.filter((route) => route.nav);
