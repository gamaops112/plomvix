import { describe, it, expect } from 'vitest';
import { flattenStats, type StatCardItem } from '../admin/statsFlatten';

describe('flattenStats', () => {
  it('flattens nested objects', () => {
    const input = { hot_tier: { recordCount: 1500, sizeBytes: 1024000 } };
    const result = flattenStats(input, 'Hot Tier');

    expect(result.length).toBeGreaterThanOrEqual(2);
    expect(result.some((s: StatCardItem) => s.label === 'Record Count')).toBe(true);
    expect(result.some((s: StatCardItem) => s.group === 'Hot Tier')).toBe(true);
  });

  it('handles arrays as lengths', () => {
    const input = { items: [1, 2, 3] };
    const result = flattenStats(input);

    const arrayItem = result.find((s: StatCardItem) => s.key.includes('items'));
    expect(arrayItem).toBeDefined();
    expect(arrayItem?.value).toBe('3');
  });

  it('handles booleans', () => {
    const input = { debug_enabled: true };
    const result = flattenStats(input);

    const debugItem = result.find((s: StatCardItem) => s.key.includes('debug'));
    expect(debugItem).toBeDefined();
    expect(debugItem?.value).toBe('Yes');
  });

  it('stops at depth limit', () => {
    const input = { a: { b: { c: { d: { e: 'too deep' } } } } };
    const result = flattenStats(input);

    const deepItems = result.filter((s: StatCardItem) =>
      s.key.split('.').length > 5
    );
    expect(deepItems.length).toBe(0);
  });

  it('skips null and undefined values', () => {
    const input = { keep: 42, skip_null: null, skip_undef: undefined };
    const result = flattenStats(input);

    expect(result.length).toBe(1);
    expect(result[0].value).toBe('42');
  });

  it('returns empty array for empty input', () => {
    expect(flattenStats({})).toEqual([]);
  });

  it('assigns groups', () => {
    const input = { wal_segments: 5 };
    const result = flattenStats(input, 'WAL');

    expect(result.length).toBe(1);
    expect(result[0].group).toBe('WAL');
  });
});
