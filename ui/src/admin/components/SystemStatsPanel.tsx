import * as React from "react"

import { Button } from "@/components/ui/button"
import { LoadingState } from "./LoadingState"
import { ErrorState } from "./ErrorState"
import { AdminSection } from "./AdminSection"
import { StatCard } from "./StatCard"
import { useAdminStats } from "../hooks/useAdminStats"
import { flattenStats, type StatCardItem } from "../statsFlatten"
import { formatDateTime, formatDuration } from "../format"

const GROUP_LABELS: Record<string, string> = {
  wal: "WAL",
  hot_tier: "Hot Tier",
  cold_tier: "Cold Tier",
  runtime: "Runtime",
}

function groupLabel(key: string): string {
  return GROUP_LABELS[key] ?? key
    .replace(/_/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

function buildInfoItems(info: Record<string, unknown> | null): StatCardItem[] {
  if (!info) return []
  const items: StatCardItem[] = []

  if (info.version) {
    items.push({ key: "version", label: "Version", value: String(info.version), group: "" })
  }
  if (info.git_commit) {
    items.push({ key: "git_commit", label: "Git Commit", value: String(info.git_commit).slice(0, 7), group: "" })
  }
  if (info.go_version) {
    items.push({ key: "go_version", label: "Go Version", value: String(info.go_version), group: "" })
  }
  if (info.uptime_seconds !== undefined && info.uptime_seconds !== null) {
    items.push({ key: "uptime", label: "Uptime", value: formatDuration(info.uptime_seconds), group: "" })
  }
  if (info.build_time) {
    items.push({ key: "build_time", label: "Build Time", value: formatDateTime(String(info.build_time)), group: "" })
  }

  return items
}

function buildStatItems(stats: Record<string, unknown> | null): StatCardItem[] {
  if (!stats) return []
  const all: StatCardItem[] = []

  for (const [key, value] of Object.entries(stats)) {
    if (value === null || value === undefined) continue
    if (typeof value === "object" && !Array.isArray(value)) {
      const group = groupLabel(key)
      all.push(...flattenStats(value as Record<string, unknown>, group))
    }
  }

  return all
}

export function SystemStatsPanel(): React.ReactElement {
  const { stats, info, loading, refreshing, error, lastLoadedAt, reload } = useAdminStats(30000)

  const actions = (
    <div className="flex items-center gap-3">
      {lastLoadedAt && (
        <span className="text-xs text-muted-foreground">
          Updated {formatDateTime(lastLoadedAt.toISOString())}
        </span>
      )}
      <Button variant="outline" size="xs" onClick={reload} disabled={refreshing}>
        Refresh
      </Button>
    </div>
  )

  if (loading) {
    return (
      <AdminSection title="System Stats">
        <LoadingState />
      </AdminSection>
    )
  }

  if (error) {
    return (
      <AdminSection title="System Stats">
        <ErrorState message={error} onRetry={reload} />
      </AdminSection>
    )
  }

  const infoItems = buildInfoItems(info)
  const statItems = buildStatItems(stats)

  const grouped = new Map<string, StatCardItem[]>()
  for (const item of statItems) {
    const g = item.group || "General"
    if (!grouped.has(g)) grouped.set(g, [])
    grouped.get(g)!.push(item)
  }

  return (
    <AdminSection title="System Stats" actions={actions}>
      {refreshing && (
        <div className="flex items-center gap-2 -mt-2 mb-4 text-xs text-muted-foreground">
          <span className="inline-block size-2 rounded-full bg-amber-400 animate-pulse" />
          Refreshing…
        </div>
      )}

      {infoItems.length > 0 && (
        <div className="mb-6">
          <h3 className="text-sm font-semibold mb-3 text-foreground">Build &amp; Runtime Info</h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {infoItems.map((item) => (
              <StatCard key={item.key} label={item.label} value={item.value} />
            ))}
          </div>
        </div>
      )}

      {Array.from(grouped.entries()).map(([group, items]) => (
        <div key={group} className="mb-6 last:mb-0">
          <h3 className="text-sm font-semibold mb-3 text-foreground">{group}</h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {items.map((item) => (
              <StatCard key={item.key} label={item.label} value={item.value} />
            ))}
          </div>
        </div>
      ))}
    </AdminSection>
  )
}
