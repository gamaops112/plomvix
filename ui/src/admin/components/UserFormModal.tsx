import * as React from "react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"

import { Modal } from "../../components/Modal"
import type { AdminUser, CreateUserRequest, UpdateUserRequest } from "../types"
import { titleCase } from "../format"

interface UserFormModalProps {
  open: boolean
  mode: "create" | "edit"
  user?: AdminUser
  submitting: boolean
  onSubmit: (input: CreateUserRequest | UpdateUserRequest) => Promise<void>
  onClose: () => void
}

export function UserFormModal({
  open,
  mode,
  user,
  submitting,
  onSubmit,
  onClose,
}: UserFormModalProps): React.ReactElement {
  const [username, setUsername] = React.useState("")
  const [password, setPassword] = React.useState("")
  const [usernameError, setUsernameError] = React.useState("")
  const [passwordError, setPasswordError] = React.useState("")

  const isCreate = mode === "create"

  React.useEffect(() => {
    if (open) {
      setUsername(user?.username ?? "")
      setPassword("")
      setUsernameError("")
      setPasswordError("")
    }
  }, [open, user])

  function validate(): boolean {
    let valid = true

    const trimmedUsername = username.trim()
    if (!trimmedUsername) {
      setUsernameError("Username is required")
      valid = false
    } else {
      setUsernameError("")
    }

    if (isCreate && !password) {
      setPasswordError("Password is required")
      valid = false
    } else {
      setPasswordError("")
    }

    return valid
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!validate()) return

    const input: Record<string, string> = {}
    const trimmedUsername = username.trim()

    if (isCreate || trimmedUsername !== user?.username) {
      input.username = trimmedUsername
    }
    if (password) {
      input.password = password
    }

    await onSubmit(input as CreateUserRequest | UpdateUserRequest)
    setPassword("")
  }

  return (
    <Modal
      open={open}
      title={isCreate ? "Create User" : "Edit User"}
      onClose={onClose}
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={submitting} onClick={handleSubmit}>
            {isCreate ? "Create" : "Save"}
          </Button>
        </>
      }
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="user-username">Username</Label>
          <Input
            id="user-username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            disabled={submitting}
          />
          {usernameError && (
            <p className="text-sm text-destructive">{usernameError}</p>
          )}
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor="user-password">
            Password{isCreate ? "" : " (optional)"}
          </Label>
          <Input
            id="user-password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={isCreate ? "" : "Leave blank to keep unchanged"}
            disabled={submitting}
          />
          {passwordError && (
            <p className="text-sm text-destructive">{passwordError}</p>
          )}
        </div>

        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">Role:</span>
          <Badge>{titleCase("admin")}</Badge>
        </div>
      </form>
    </Modal>
  )
}
