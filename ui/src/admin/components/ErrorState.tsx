import * as React from "react"

import { Button } from "@/components/ui/button"

interface ErrorStateProps {
  title?: string
  message: string
  onRetry?: () => void
}

export function ErrorState({
  title = "Error",
  message,
  onRetry,
}: ErrorStateProps): React.ReactElement {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <span className="text-3xl text-destructive">!</span>
      <h3 className="mt-4 text-lg font-semibold text-foreground">
        {title}
      </h3>
      <p className="mt-1 max-w-md text-sm text-muted-foreground">
        {message}
      </p>
      {onRetry && (
        <Button variant="outline" onClick={onRetry} className="mt-6">
          Try again
        </Button>
      )}
    </div>
  )
}
