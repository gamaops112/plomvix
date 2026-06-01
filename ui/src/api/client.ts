export class ApiError extends Error {
  status: number;
  code?: string;

  constructor(status: number, message: string, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

let onSessionExpired: (() => void) | null = null;

export function setSessionExpiredHandler(fn: () => void) {
  onSessionExpired = fn;
}

async function parseBody(res: Response): Promise<unknown> {
  const ct = res.headers.get('content-type') || '';
  if (ct.includes('application/json')) {
    return res.json();
  }
  return null;
}

async function extractError(res: Response): Promise<ApiError> {
  let message = res.statusText || 'Request failed';
  let code: string | undefined;
  try {
    const body = await parseBody(res);
    if (body && typeof body === 'object') {
      const b = body as Record<string, unknown>;
      const err = b.error as Record<string, unknown> | undefined;
      if (err) {
        message = (err.message as string) || message;
        code = (err.code as string) || code;
      }
      if (b.details && Array.isArray(b.details)) {
        const details = b.details as string[];
        if (details.length > 0) {
          message = details[0];
        }
      }
    }
  } catch {
    // Use status text
  }
  return new ApiError(res.status, message, code);
}

export async function apiRequest<T>(path: string, init?: RequestInit, retry = true): Promise<T> {
  const res = await fetch(path, {
    ...init,
    credentials: 'include',
  });

  if (res.ok) {
    const body = await parseBody(res);
    if (body && typeof body === 'object') {
      const b = body as Record<string, unknown>;
      if (b.data !== undefined) return b.data as T;
      if (b.status !== undefined) return body as T;
    }
    return body as T;
  }

  if (res.status === 401 && retry) {
    try {
      const refreshRes = await fetch('/auth/refresh', {
        method: 'POST',
        credentials: 'include',
      });
      if (refreshRes.ok) {
        return apiRequest<T>(path, init, false);
      }
    } catch {
      // Refresh failed
    }
    // Refresh failed — signal session expiry
    if (onSessionExpired) {
      onSessionExpired();
    }
  }

  throw await extractError(res);
}

export async function apiGet<T>(path: string): Promise<T> {
  return apiRequest<T>(path);
}

export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method: 'POST',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  };
  return apiRequest<T>(path, init);
}

export async function apiPut<T>(path: string, body?: unknown): Promise<T> {
  const init: RequestInit = {
    method: 'PUT',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  };
  return apiRequest<T>(path, init);
}

export async function apiDelete<T>(path: string): Promise<T> {
  const init: RequestInit = { method: 'DELETE' };
  return apiRequest<T>(path, init);
}
