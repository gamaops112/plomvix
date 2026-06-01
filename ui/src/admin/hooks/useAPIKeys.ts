import { useState, useCallback } from 'react';
import { getAPIKeyStatus, generateAPIKey, revokeAPIKey } from '../adminApi';
import type { AdminUser, APIKeyStatus } from '../types';
import { useAppEvents } from '../../events/AppEventProvider';

export function useAPIKeys(_users: AdminUser[]) {
  const [statusByUserId, setStatusByUserId] = useState<Record<string, APIKeyStatus>>({});
  const [generatedKeyByUserId, setGeneratedKeyByUserId] = useState<Record<string, string>>({});
  const [loadingByUserId, setLoadingByUserId] = useState<Record<string, boolean>>({});
  const [error, setError] = useState<string | null>(null);
  const { emit } = useAppEvents();

  const setLoading = useCallback((userId: string, value: boolean) => {
    setLoadingByUserId((prev) => ({ ...prev, [userId]: value }));
  }, []);

  const loadStatus = useCallback(
    async (userId: string) => {
      try {
        setError(null);
        setLoading(userId, true);
        const status = await getAPIKeyStatus(userId);
        setStatusByUserId((prev) => ({ ...prev, [userId]: status }));
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to load API key status';
        setError(message);
        emit({ type: 'toast:add', payload: { kind: 'error', title: message } });
      } finally {
        setLoading(userId, false);
      }
    },
    [emit, setLoading]
  );

  const loadAllStatuses = useCallback(async () => {
    const userIds = _users.map((u) => u.id);
    await Promise.all(userIds.map((id) => loadStatus(id)));
  }, [_users, loadStatus]);

  const generate = useCallback(
    async (userId: string) => {
      try {
        setError(null);
        setLoading(userId, true);
        const result = await generateAPIKey(userId);
        setGeneratedKeyByUserId((prev) => ({ ...prev, [userId]: result.api_key }));
        setStatusByUserId((prev) => ({ ...prev, [userId]: { user_id: userId, has_api_key: true } }));
        emit({ type: 'toast:add', payload: { kind: 'success', title: 'API key generated' } });
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to generate API key';
        setError(message);
        emit({ type: 'toast:add', payload: { kind: 'error', title: message } });
      } finally {
        setLoading(userId, false);
      }
    },
    [emit, setLoading]
  );

  const revoke = useCallback(
    async (userId: string) => {
      try {
        setError(null);
        setLoading(userId, true);
        await revokeAPIKey(userId);
        setStatusByUserId((prev) => ({ ...prev, [userId]: { user_id: userId, has_api_key: false } }));
        setGeneratedKeyByUserId((prev) => {
          const next = { ...prev };
          delete next[userId];
          return next;
        });
        emit({ type: 'toast:add', payload: { kind: 'success', title: 'API key revoked' } });
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to revoke API key';
        setError(message);
        emit({ type: 'toast:add', payload: { kind: 'error', title: message } });
      } finally {
        setLoading(userId, false);
      }
    },
    [emit, setLoading]
  );

  const clearGeneratedKey = useCallback((userId: string) => {
    setGeneratedKeyByUserId((prev) => {
      const next = { ...prev };
      delete next[userId];
      return next;
    });
  }, []);

  return {
    statusByUserId,
    generatedKeyByUserId,
    loadingByUserId,
    error,
    loadStatus,
    loadAllStatuses,
    generate,
    revoke,
    clearGeneratedKey,
  };
}
