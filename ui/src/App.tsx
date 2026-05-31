import { Shell } from './components/layout/Shell';
import { AppRoutes } from './app/AppRoutes';
import { ToastViewport } from './components/feedback/ToastViewport';

export function App() {
  return (
    <>
      <Shell>
        <AppRoutes />
      </Shell>
      <ToastViewport />
    </>
  );
}
