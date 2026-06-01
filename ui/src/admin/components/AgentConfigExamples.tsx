import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"

const CURL_EXAMPLE = `curl -H "X-API-Key: YOUR_PLOMVIX_API_KEY" \\
     -H "Content-Type: application/json" \\
     -d '{"stream":"app","level":"info","message":"hello"}' \\
     http://localhost:8080/api/ingest`

const TELEGRAF_EXAMPLE = `[[outputs.http]]
  url = "http://localhost:8080/api/ingest"
  method = "POST"
  data_format = "json"
  [outputs.http.headers]
    X-API-Key = "YOUR_PLOMVIX_API_KEY"
    Content-Type = "application/json"`

const VECTOR_EXAMPLE = `[sinks.plomvix]
  type = "http"
  inputs = ["*"]
  uri = "http://localhost:8080/api/ingest"
  encoding.codec = "json"
  [sinks.plomvix.request.headers]
    X-API-Key = "YOUR_PLOMVIX_API_KEY"`

const FLUENTBIT_EXAMPLE = `[OUTPUT]
    Name  http
    Match *
    Host  localhost
    Port  8080
    URI   /api/ingest
    Format json
    Header X-API-Key YOUR_PLOMVIX_API_KEY`

export function AgentConfigExamples(): React.ReactElement {
  return (
    <div className="mt-8 space-y-6">
      <div className="flex items-center gap-3">
        <h3 className="text-lg font-semibold">Agent Configuration Examples</h3>
        <Badge variant="outline" className="border-yellow-600/50 text-yellow-700 dark:text-yellow-400">
          Sprint 14
        </Badge>
      </div>

      <Alert className="border-yellow-600/50 bg-yellow-50 dark:bg-yellow-950/30">
        <AlertDescription className="text-yellow-800 dark:text-yellow-400">
          Sprint 14 API keys grant full API access. Scopes are not available yet.
        </AlertDescription>
      </Alert>

      <div className="grid gap-4 sm:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>curl</CardTitle>
            <CardDescription>
              Send a log entry from the command line using the API key header.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto rounded-lg bg-muted p-4">
              <code className="whitespace-pre font-mono text-sm">{CURL_EXAMPLE}</code>
            </pre>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Telegraf</CardTitle>
            <CardDescription>
              Configure the HTTP output plugin for Telegraf.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto rounded-lg bg-muted p-4">
              <code className="whitespace-pre font-mono text-sm">{TELEGRAF_EXAMPLE}</code>
            </pre>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Vector</CardTitle>
            <CardDescription>
              Configure the HTTP sink for Vector.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto rounded-lg bg-muted p-4">
              <code className="whitespace-pre font-mono text-sm">{VECTOR_EXAMPLE}</code>
            </pre>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Fluent Bit</CardTitle>
            <CardDescription>
              Configure the HTTP output plugin for Fluent Bit.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto rounded-lg bg-muted p-4">
              <code className="whitespace-pre font-mono text-sm">{FLUENTBIT_EXAMPLE}</code>
            </pre>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
