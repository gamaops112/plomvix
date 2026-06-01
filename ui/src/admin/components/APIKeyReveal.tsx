import { useState, useCallback } from "react"
import { Button } from "@/components/ui/button"
import { useAppEvents } from "../../events/AppEventProvider"

interface APIKeyRevealProps {
  apiKey: string
  onClear: () => void
}

export function APIKeyReveal(props: APIKeyRevealProps): React.ReactElement {
  const { apiKey, onClear } = props
  const [show, setShow] = useState(false)
  const [copied, setCopied] = useState(false)
  const { emit } = useAppEvents()

  const masked =
    apiKey.length > 8
      ? `${apiKey.slice(0, 4)}********${apiKey.slice(-4)}`
      : "********"

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(apiKey)
      setCopied(true)
      emit({
        type: "toast:add",
        payload: { kind: "success", title: "API key copied to clipboard" },
      })
      setTimeout(() => setCopied(false), 2000)
    } catch {
      emit({
        type: "toast:add",
        payload: { kind: "error", title: "Failed to copy API key" },
      })
    }
  }, [apiKey, emit])

  return (
    <div className="rounded-lg border border-yellow-600/50 bg-yellow-50 p-4 dark:bg-yellow-950/30">
      <p className="mb-2 text-sm font-medium text-yellow-800 dark:text-yellow-400">
        This is the only time the plaintext API key will be shown.
      </p>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <code className="flex-1 break-all rounded bg-yellow-100 px-3 py-2 font-mono text-sm text-yellow-900 dark:bg-yellow-900/40 dark:text-yellow-200">
          {show ? apiKey : masked}
        </code>
        <div className="flex shrink-0 gap-1">
          <Button variant="outline" size="sm" onClick={() => setShow((p) => !p)}>
            {show ? "Hide" : "Show"}
          </Button>
          <Button variant="outline" size="sm" onClick={handleCopy}>
            {copied ? "Copied!" : "Copy"}
          </Button>
          <Button variant="ghost" size="sm" onClick={onClear}>
            Clear
          </Button>
        </div>
      </div>
    </div>
  )
}
