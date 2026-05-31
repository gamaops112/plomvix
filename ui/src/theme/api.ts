import type { Theme } from './types';

const BASE = '';

async function unwrap<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let detail = res.statusText;
    try {
      const body = await res.json();
      detail = body.details?.[0] ?? body.error ?? detail;
    } catch {
      // use status text
    }
    throw new Error(detail);
  }
  const envelope = await res.json();
  return envelope.data as T;
}

export async function fetchTheme(): Promise<Theme> {
  return unwrap<Theme>(
    await fetch(`${BASE}/api/theme`, { credentials: 'same-origin' })
  );
}

export async function saveTheme(theme: Theme): Promise<Theme> {
  return unwrap<Theme>(
    await fetch(`${BASE}/api/theme`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      body: JSON.stringify(theme),
    })
  );
}

export async function resetTheme(): Promise<Theme> {
  return unwrap<Theme>(
    await fetch(`${BASE}/api/theme/reset`, {
      method: 'POST',
      credentials: 'same-origin',
    })
  );
}

export async function exportTheme(): Promise<Blob> {
  const res = await fetch(`${BASE}/api/theme/export`, {
    credentials: 'same-origin',
  });
  if (!res.ok) {
    throw new Error('Theme export failed');
  }
  return res.blob();
}
