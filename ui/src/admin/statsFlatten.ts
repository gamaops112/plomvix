import { titleCase, formatNumber } from './format'

export interface StatCardItem {
  key: string
  label: string
  value: string
  group: string
}

const numberFormat = new Intl.NumberFormat('en-US')

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return '—'
  if (typeof value === 'boolean') return value ? 'Yes' : 'No'
  if (typeof value === 'number') return numberFormat.format(value)
  if (typeof value === 'string') {
    const num = Number(value)
    if (value.trim() !== '' && Number.isFinite(num)) return numberFormat.format(num)
    return value
  }
  if (Array.isArray(value)) return String(value.length)
  return '—'
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function flatten(
  input: Record<string, unknown>,
  prefix: string,
  group: string,
  depth: number,
  results: StatCardItem[],
): void {
  if (depth > 3) return

  for (const [k, v] of Object.entries(input)) {
    const key = prefix ? `${prefix}_${k}` : k

    if (isPlainObject(v) && depth < 3) {
      flatten(v, key, group, depth + 1, results)
      continue
    }

    if (v === null || v === undefined) continue

    results.push({
      key,
      label: titleCase(k),
      value: isPlainObject(v) ? '—' : formatValue(v),
      group,
    })
  }
}

export function flattenStats(
  input: Record<string, unknown>,
  group?: string,
): StatCardItem[] {
  const results: StatCardItem[] = []
  flatten(input, '', group ?? '', 1, results)
  return results
}
