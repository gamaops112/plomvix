import * as React from "react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"

import { Modal } from "../../components/Modal"
import type { AdminUser } from "../types"

interface DeleteUserModalProps {
  open: boolean
  user: AdminUser | null
  submitting: boolean
  onConfirm: () => Promise<void>
  onClose: () => void
}

export function DeleteUserModal({
  open,
  user,
  submitting,
  onConfirm,
  onClose,
}: DeleteUserModalProps): React.ReactElement {
  const [confirmation, setConfirmation] = React.useState("")

  React.useEffect(() => {
    if (open) {
      setConfirmation("")
    }
  }, [open])

  if (!user) {
    return <Modal open={false} title="" onClose={() => {}} children={null} />
  }

  const matches = confirmation === user.username

  async function handleConfirm() {
    if (!matches) return
    await onConfirm()
    setConfirmation("")
  }

  return (
    <Modal
      open={open}
      title="Delete User"
      onClose={onClose}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={!matches || submitting}
            onClick={handleConfirm}
          >
            Delete
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <p className="text-sm text-muted-foreground">
          This action is permanent and cannot be undone.
        </p>

        <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3">
          <p className="text-sm">
            You are about to delete user{" "}
            <strong>{user.username}</strong>.
          </p>
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor="delete-confirmation">
            Type <strong>{user.username}</strong> to confirm
          </Label>
          <Input
            id="delete-confirmation"
            value={confirmation}
            onChange={(e) => setConfirmation(e.target.value)}
            placeholder={user.username}
            disabled={submitting}
          />
        </div>
      </div>
    </Modal>
  )
}
