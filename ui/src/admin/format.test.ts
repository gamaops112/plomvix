import { describe, it, expect } from 'vitest';
import { formatDateTime, formatDuration, formatNumber, titleCase } from '../admin/format';

describe('formatDateTime', () => {
  it('returns — for null', () => {
    expect(formatDateTime(null)).toBe('—');
  });

  it('returns — for undefined', () => {
    expect(formatDateTime(undefined)).toBe('—');
  });

  it('returns — for empty string', () => {
    expect(formatDateTime('')).toBe('—');
  });

  it('returns — for invalid date', () => {
    expect(formatDateTime('not-a-date')).toBe('—');
  });

  it('formats valid ISO date', () => {
    const result = formatDateTime('2024-01-15T12:30:00Z');
    expect(result).not.toBe('—');
    expect(result).toContain('2024');
  });
});

describe('formatDuration', () => {
  it('returns — for invalid values', () => {
    expect(formatDuration(null)).toBe('—');
    expect(formatDuration(undefined)).toBe('—');
    expect(formatDuration('abc')).toBe('—');
  });

  it('formats seconds', () => {
    expect(formatDuration(45)).toBe('45s');
  });

  it('formats minutes and seconds', () => {
    expect(formatDuration(150)).toBe('2m 30s');
  });

  it('formats hours and minutes', () => {
    expect(formatDuration(3600)).toBe('1h 0m');
  });

  it('handles string numbers', () => {
    expect(formatDuration('7200')).toBe('2h 0m');
  });
});

describe('formatNumber', () => {
  it('returns — for invalid values', () => {
    expect(formatNumber(null)).toBe('—');
    expect(formatNumber(undefined)).toBe('—');
    expect(formatNumber('abc')).toBe('—');
  });

  it('formats numbers with commas', () => {
    const result = formatNumber(1234567);
    expect(result).toContain('1');
    expect(result).not.toBe('—');
  });

  it('handles string numbers', () => {
    expect(formatNumber('42')).not.toBe('—');
  });
});

describe('titleCase', () => {
  it('converts snake_case', () => {
    expect(titleCase('created_at')).toBe('Created At');
  });

  it('converts camelCase', () => {
    expect(titleCase('hotTierBytes')).toBe('Hot Tier Bytes');
  });

  it('handles UPPER_CASE', () => {
    expect(titleCase('WAL_SEGMENTS')).toBe('Wal Segments');
  });

  it('handles single word', () => {
    expect(titleCase('node')).toBe('Node');
  });
});
