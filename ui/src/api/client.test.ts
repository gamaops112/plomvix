import { describe, it, expect, vi, beforeEach } from 'vitest';
import { apiGet, apiPost, ApiError } from './client';

beforeEach(() => {
  vi.restoreAllMocks();
});

describe('apiGet', () => {
  it('sends requests with credentials: include', async () => {
    const spy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ status: 'ok', data: { message: 'hello' } }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    await apiGet('/test');
    expect(spy).toHaveBeenCalledWith('/test', expect.objectContaining({
      credentials: 'include',
    }));
  });

  it('parses JSON success response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ status: 'ok', data: { value: 42 } }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    const result = await apiGet<{ value: number }>('/test');
    expect(result).toEqual({ value: 42 });
  });

  it('throws ApiError on non-2xx response', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ status: 'error', error: { code: 'NOT_FOUND', message: 'not found' } }), {
        status: 404,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    await expect(apiGet('/missing')).rejects.toThrow(ApiError);
  });
});

describe('apiPost', () => {
  it('sends JSON body with Content-Type header', async () => {
    const spy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify({ status: 'ok', data: {} }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );

    await apiPost('/submit', { key: 'value' });
    const call = spy.mock.calls[0];
    const init = call[1] as RequestInit;
    expect(init.method).toBe('POST');
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' });
    expect(init.body).toBe(JSON.stringify({ key: 'value' }));
  });
});
