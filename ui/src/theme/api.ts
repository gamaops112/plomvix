import type { Theme } from './types';
import { apiGet, apiPut, apiPost } from '../api/client';

export async function fetchTheme(): Promise<Theme> {
  return apiGet<Theme>('/api/theme');
}

export async function saveTheme(theme: Theme): Promise<Theme> {
  return apiPut<Theme>('/api/theme', theme);
}

export async function resetTheme(): Promise<Theme> {
  return apiPost<Theme>('/api/theme/reset');
}

export async function exportTheme(): Promise<Blob> {
  const res = await fetch('/api/theme/export', { credentials: 'include' });
  if (!res.ok) {
    throw new Error('Theme export failed');
  }
  return res.blob();
}
