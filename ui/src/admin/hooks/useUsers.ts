import { useState, useEffect, useCallback } from 'react';
import { listUsers, createUser, updateUser, deleteUser } from '../adminApi';
import type { AdminUser, CreateUserRequest, UpdateUserRequest } from '../types';
import { useAppEvents } from '../../events/AppEventProvider';

export function useUsers() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const { emit } = useAppEvents();

  const load = useCallback(async () => {
    try {
      setError(null);
      const result = await listUsers();
      const sorted = [...result].sort(
        (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
      );
      setUsers(sorted);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load users';
      setError(message);
      emit({ type: 'toast:add', payload: { kind: 'error', title: message } });
    }
  }, [emit]);

  useEffect(() => {
    let cancelled = false;
    const fetch = async () => {
      setLoading(true);
      await load();
      if (!cancelled) setLoading(false);
    };
    fetch();
    return () => {
      cancelled = true;
    };
  }, [load]);

  const create = useCallback(
    async (input: CreateUserRequest) => {
      try {
        setError(null);
        await createUser(input);
        emit({ type: 'toast:add', payload: { kind: 'success', title: 'User created' } });
        await load();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to create user';
        setError(message);
        emit({ type: 'toast:add', payload: { kind: 'error', title: message } });
      }
    },
    [emit, load]
  );

  const update = useCallback(
    async (id: string, input: UpdateUserRequest) => {
      try {
        setError(null);
        await updateUser(id, input);
        emit({ type: 'toast:add', payload: { kind: 'success', title: 'User updated' } });
        await load();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to update user';
        setError(message);
        emit({ type: 'toast:add', payload: { kind: 'error', title: message } });
      }
    },
    [emit, load]
  );

  const remove = useCallback(
    async (id: string) => {
      try {
        setError(null);
        await deleteUser(id);
        emit({ type: 'toast:add', payload: { kind: 'success', title: 'User deleted' } });
        await load();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to delete user';
        setError(message);
        emit({ type: 'toast:add', payload: { kind: 'error', title: message } });
      }
    },
    [emit, load]
  );

  return { users, loading, error, reload: load, create, update, remove };
}
