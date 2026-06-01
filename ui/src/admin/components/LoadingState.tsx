import * as React from "react"

import { Skeleton } from "@/components/ui/skeleton"

interface LoadingStateProps {
  label?: string
}

export function LoadingState({
  label,
}: LoadingStateProps): React.ReactElement {
  return (
    <div className="flex flex-col items-center justify-center py-16">
      <div className="flex w-full max-w-md flex-col gap-4">
        <Skeleton className="h-6 w-3/4" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-5/6" />
        <Skeleton className="h-4 w-2/3" />
      </div>
      {label && (
        <p className="mt-6 text-sm text-muted-foreground">{label}</p>
      )}
    </div>
  )
}
