import { useState, useEffect, useCallback, useRef } from 'react';
import { getAdminStats, getAdminInfo } from '../adminApi';
import type { AdminStats, AdminInfo } from '../types';
import { useAppEvents } from '../../events/AppEventProvider';

export function useAdminStats(autoRefreshMs = 30000) {
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [info, setInfo] = useState<AdminInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastLoadedAt, setLastLoadedAt] = useState<Date | null>(null);
  const { emit } = useAppEvents();
  const errorEmittedRef = useRef(false);

  const fetchData = useCallback(
    async (isRefresh: boolean) => {
      try {
        if (isRefresh) {
          setRefreshing(true);
        }
        const [s, i] = await Promise.all([getAdminStats(), getAdminInfo()]);
        setStats(s);
        setInfo(i);
        setError(null);
        setLastLoadedAt(new Date());
        errorEmittedRef.current = false;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to load admin data';
        if (isRefresh || !errorEmittedRef.current) {
          setError(message);
          emit({ type: 'toast:add', payload: { kind: 'error', title: message } });
          errorEmittedRef.current = true;
        }
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    },
    [emit]
  );

  useEffect(() => {
    fetchData(false);

    const id = window.setInterval(() => {
      fetchData(true);
    }, autoRefreshMs);

    return () => {
      window.clearInterval(id);
    };
  }, [fetchData, autoRefreshMs]);

  const reload = useCallback(() => {
    fetchData(true);
  }, [fetchData]);

  return { stats, info, loading, refreshing, error, lastLoadedAt, reload };
}
