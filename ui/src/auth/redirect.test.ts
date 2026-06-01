import { describe, it, expect } from 'vitest';
import { safeNextPath } from './redirect';

describe('safeNextPath', () => {
  it('returns /app/explore for null', () => {
    expect(safeNextPath(null)).toBe('/app/explore');
  });

  it('returns /app/explore for empty string', () => {
    expect(safeNextPath('')).toBe('/app/explore');
  });

  it('accepts /app/explore', () => {
    expect(safeNextPath('/app/explore')).toBe('/app/explore');
  });

  it('accepts /app', () => {
    expect(safeNextPath('/app')).toBe('/app');
  });

  it('accepts /dev/design', () => {
    expect(safeNextPath('/dev/design')).toBe('/dev/design');
  });

  it('rejects external absolute URL', () => {
    expect(safeNextPath('https://evil.com')).toBe('/app/explore');
  });

  it('rejects protocol-relative URL', () => {
    expect(safeNextPath('//evil.com')).toBe('/app/explore');
  });

  it('handles URL-encoded paths', () => {
    expect(safeNextPath('%2Fapp%2Fexplore')).toBe('/app/explore');
  });

  it('rejects malformed URI', () => {
    expect(safeNextPath('%ZZ')).toBe('/app/explore');
  });
});
