import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react';
import type { ReactNode } from 'react';
import type { Theme, ThemeMode } from './types';
import { defaultTheme } from './defaultTheme';
import { applyTheme } from './cssVariables';
import { fetchTheme, saveTheme as apiSaveTheme, resetTheme as apiResetTheme } from './api';
import { useAppEvents } from '../events/AppEventProvider';

interface ThemeContextValue {
  theme: Theme;
  draft: Theme;
  mode: ThemeMode;
  loading: boolean;
  error: string | null;
  setMode: (mode: ThemeMode) => void;
  setDraftTheme: (theme: Theme) => void;
  saveDraft: () => Promise<void>;
  resetToDefault: () => Promise<void>;
  reloadTheme: () => Promise<void>;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }): React.ReactElement {
  const { emit } = useAppEvents();
  const [theme, setTheme] = useState<Theme>(defaultTheme);
  const [draft, setDraft] = useState<Theme>(defaultTheme);
  const [mode, setMode] = useState<ThemeMode>('light');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const reloadTheme = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const t = await fetchTheme();
      setTheme(t);
      setDraft(t);
      setMode(t.mode);
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Unknown theme error';
      setError(msg);
      emit({ type: 'toast:add', payload: { kind: 'error', title: 'Theme load failed', message: msg } });
    } finally {
      setLoading(false);
    }
  }, [emit]);

  useEffect(() => {
    reloadTheme();
  }, [reloadTheme]);

  useEffect(() => {
    applyTheme(draft, mode);
    const root = document.documentElement;
    if (mode === 'dark') {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }
    root.dataset.theme = mode;
  }, [draft, mode]);

  const setDraftTheme = useCallback((t: Theme) => {
    setDraft(t);
  }, []);

  const saveDraft = useCallback(async () => {
    try {
      const saved = await apiSaveTheme(draft);
      setTheme(saved);
      setDraft(saved);
      setMode(saved.mode);
      emit({ type: 'toast:add', payload: { kind: 'success', title: 'Theme saved' } });
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Save failed';
      emit({ type: 'toast:add', payload: { kind: 'error', title: 'Admin login required to save theme', message: msg } });
    }
  }, [draft, emit]);

  const resetToDefault = useCallback(async () => {
    try {
      const t = await apiResetTheme();
      setTheme(t);
      setDraft(t);
      setMode(t.mode);
      emit({ type: 'toast:add', payload: { kind: 'success', title: 'Theme reset to defaults' } });
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'Reset failed';
      emit({ type: 'toast:add', payload: { kind: 'error', title: 'Admin login required to reset theme', message: msg } });
    }
  }, [emit]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      theme,
      draft,
      mode,
      loading,
      error,
      setMode: (m: ThemeMode) => {
        setMode(m);
        setDraft((prev) => ({ ...prev, mode: m }));
      },
      setDraftTheme,
      saveDraft,
      resetToDefault,
      reloadTheme,
    }),
    [theme, draft, mode, loading, error, setDraftTheme, saveDraft, resetToDefault, reloadTheme]
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside ThemeProvider');
  return ctx;
}
