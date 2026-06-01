import type { AdminUser, CreateUserRequest, UpdateUserRequest, APIKeyStatus, GeneratedAPIKey, AdminStats, AdminInfo } from './types';
import { apiGet, apiPost, apiPatch, apiDelete } from '../api/client';

function asAdminUser(input: unknown): AdminUser {
  const u = input as Record<string, unknown>;
  if (u && typeof u === 'object' && 'user' in u) {
    const inner = u.user as Record<string, unknown>;
    if (inner && typeof inner === 'object') {
      return inner as unknown as AdminUser;
    }
  }
  return input as AdminUser;
}

function asAdminUsers(input: unknown): AdminUser[] {
  if (Array.isArray(input)) return input as AdminUser[];
  const u = input as Record<string, unknown> | null;
  if (u && Array.isArray(u.users)) return u.users as AdminUser[];
  return [];
}

export async function listUsers(): Promise<AdminUser[]> {
  return apiGet<unknown>('/admin/users').then(asAdminUsers);
}

export async function createUser(input: CreateUserRequest): Promise<AdminUser> {
  return apiPost<unknown>('/admin/users', input).then(asAdminUser);
}

export async function getUser(id: string): Promise<AdminUser> {
  return apiGet<unknown>(`/admin/users/${encodeURIComponent(id)}`).then(asAdminUser);
}

export async function updateUser(id: string, input: UpdateUserRequest): Promise<AdminUser> {
  return apiPatch<unknown>(`/admin/users/${encodeURIComponent(id)}`, input).then(asAdminUser);
}

export async function deleteUser(id: string): Promise<void> {
  await apiDelete<void>(`/admin/users/${encodeURIComponent(id)}`);
}

export async function getAPIKeyStatus(userId: string): Promise<APIKeyStatus> {
  return apiGet<APIKeyStatus>(`/admin/users/${encodeURIComponent(userId)}/apikey/status`);
}

export async function generateAPIKey(userId: string): Promise<GeneratedAPIKey> {
  const raw = await apiPost<unknown>(`/admin/users/${encodeURIComponent(userId)}/apikey`);
  const r = raw as Record<string, unknown>;
  if (r && typeof r === 'object' && 'api_key' in r) {
    return { api_key: r.api_key as string };
  }
  if (r && typeof r === 'object' && 'key' in r) {
    return { api_key: r.key as string };
  }
  return raw as GeneratedAPIKey;
}

export async function revokeAPIKey(userId: string): Promise<void> {
  await apiDelete<void>(`/admin/users/${encodeURIComponent(userId)}/apikey`);
}

export async function getAdminStats(): Promise<AdminStats> {
  return apiGet<AdminStats>('/admin/stats');
}

export async function getAdminInfo(): Promise<AdminInfo> {
  return apiGet<AdminInfo>('/admin/info');
}
