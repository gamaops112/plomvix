import { createContext, ReactNode, useCallback, useContext, useMemo, useState } from 'react';
import type { AppEvent, ToastItem } from './types';

type AppEventContextValue = {
  toasts: ToastItem[];
  emit: (event: AppEvent) => void;
};

const AppEventContext = createContext<AppEventContextValue | null>(null);

export function AppEventProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const emit = useCallback((event: AppEvent) => {
    switch (event.type) {
      case 'toast:add': {
        const toast: ToastItem = {
          id: crypto.randomUUID(),
          kind: event.payload.kind ?? 'info',
          title: event.payload.title,
          message: event.payload.message,
          durationMs: event.payload.durationMs ?? 5000
        };
        setToasts((prev) => [...prev, toast]);
        window.setTimeout(() => {
          setToasts((prev) => prev.filter((t) => t.id !== toast.id));
        }, toast.durationMs);
        return;
      }
      case 'toast:remove': {
        setToasts((prev) => prev.filter((t) => t.id !== event.payload.id));
        return;
      }
      case 'app:ready': {
        return;
      }
    }
  }, []);

  const value = useMemo(() => ({ toasts, emit }), [toasts, emit]);

  return (
    <AppEventContext.Provider value={value}>
      {children}
    </AppEventContext.Provider>
  );
}

export function useAppEvents() {
  const ctx = useContext(AppEventContext);
  if (!ctx) throw new Error('useAppEvents must be used inside AppEventProvider');
  return ctx;
}
