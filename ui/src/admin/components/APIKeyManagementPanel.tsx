import { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
} from "@/components/ui/table"
import { Modal } from "../../components/Modal"
import { useAppEvents } from "../../events/AppEventProvider"
import { useAPIKeys } from "../hooks/useAPIKeys"
import { LoadingState } from "./LoadingState"
import { ErrorState } from "./ErrorState"
import { AdminSection } from "./AdminSection"
import { APIKeyReveal } from "./APIKeyReveal"
import { AgentConfigExamples } from "./AgentConfigExamples"
import type { AdminUser } from "../types"

interface APIKeyManagementPanelProps {
  users: AdminUser[]
}

export function APIKeyManagementPanel(props: APIKeyManagementPanelProps): React.ReactElement {
  const { users } = props
  const {
    statusByUserId,
    generatedKeyByUserId,
    loadingByUserId,
    error,
    loadAllStatuses,
    generate,
    revoke,
    clearGeneratedKey,
  } = useAPIKeys(users)

  const [confirmRevokeUserId, setConfirmRevokeUserId] = useState<string | null>(null)

  useEffect(() => {
    loadAllStatuses()
  }, [loadAllStatuses])

  const handleRevokeConfirm = async () => {
    if (!confirmRevokeUserId) return
    await revoke(confirmRevokeUserId)
    setConfirmRevokeUserId(null)
  }

  const resolvedUser = users.find((u) => u.id === confirmRevokeUserId)

  if (users.length === 0) {
    return (
      <AdminSection title="API Key Management">
        <p className="text-sm text-muted-foreground">No users found.</p>
      </AdminSection>
    )
  }

  const hasStatuses = Object.keys(statusByUserId).length > 0

  return (
    <AdminSection title="API Key Management">
      {!hasStatuses && !error && <LoadingState label="Loading API key statuses..." />}

      {!hasStatuses && error && <ErrorState message={error} onRetry={loadAllStatuses} />}

      {hasStatuses && (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Username</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((user) => {
                const status = statusByUserId[user.id]
                const isLoading = loadingByUserId[user.id] ?? false
                const hasKey = status?.has_api_key ?? false
                const generatedKey = generatedKeyByUserId[user.id]

                return (
                  <>
                    <TableRow key={user.id}>
                      <TableCell className="font-medium">{user.username}</TableCell>
                      <TableCell>
                        {hasKey ? (
                          <Badge variant="default">Active</Badge>
                        ) : (
                          <Badge variant="secondary">None</Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        {isLoading ? (
                          <span className="text-sm text-muted-foreground">Working...</span>
                        ) : (
                          <div className="flex items-center justify-end gap-2">
                            {!hasKey ? (
                              <Button variant="outline" size="sm" onClick={() => generate(user.id)}>
                                Generate Key
                              </Button>
                            ) : (
                              <>
                                <Button
                                  variant="outline"
                                  size="sm"
                                  onClick={() => generate(user.id)}
                                >
                                  Generate New / Rotate
                                </Button>
                                <Button
                                  variant="destructive"
                                  size="sm"
                                  onClick={() => setConfirmRevokeUserId(user.id)}
                                >
                                  Revoke
                                </Button>
                              </>
                            )}
                          </div>
                        )}
                      </TableCell>
                    </TableRow>
                    {generatedKey && (
                      <TableRow key={`${user.id}-key`}>
                        <TableCell colSpan={3} className="pt-0">
                          <APIKeyReveal
                            apiKey={generatedKey}
                            onClear={() => clearGeneratedKey(user.id)}
                          />
                        </TableCell>
                      </TableRow>
                    )}
                  </>
                )
              })}
            </TableBody>
          </Table>

          <AgentConfigExamples />

          <Modal
            open={confirmRevokeUserId !== null}
            title="Revoke API Key"
            onClose={() => setConfirmRevokeUserId(null)}
            footer={
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={() => setConfirmRevokeUserId(null)}>
                  Cancel
                </Button>
                <Button variant="destructive" onClick={handleRevokeConfirm}>
                  Confirm Revoke
                </Button>
              </div>
            }
          >
            <p className="text-sm text-muted-foreground">
              Are you sure you want to revoke the API key for{" "}
              <span className="font-medium text-foreground">
                {resolvedUser?.username}
              </span>
              ? This action cannot be undone. Any clients using this key will lose access
              immediately.
            </p>
          </Modal>
        </>
      )}
    </AdminSection>
  )
}
