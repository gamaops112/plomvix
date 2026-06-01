import * as React from "react"

import { Badge } from "@/components/ui/badge"

interface StatCardProps {
  label: string
  value: string
  group?: string
}

export function StatCard({ label, value, group }: StatCardProps): React.ReactElement {
  return (
    <div className="relative bg-card border rounded-lg p-4 flex flex-col gap-1 min-w-0">
      {group && (
        <Badge variant="secondary" className="absolute top-2 right-2 text-[10px] px-1.5 py-0 leading-normal">
          {group}
        </Badge>
      )}
      <span className="text-xs text-muted-foreground truncate pr-12">{label}</span>
      <span className="text-lg font-bold font-mono tabular-nums truncate">{value}</span>
    </div>
  )
}
