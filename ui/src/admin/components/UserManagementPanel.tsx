import * as React from "react"

import { Button } from "@/components/ui/button"

import { AdminSection } from "./AdminSection"
import { EmptyState } from "./EmptyState"
import { LoadingState } from "./LoadingState"
import { ErrorState } from "./ErrorState"
import { UserFormModal } from "./UserFormModal"
import { DeleteUserModal } from "./DeleteUserModal"
import { UsersTable } from "./UsersTable"

import { useUsers } from "../hooks/useUsers"
import { useAuth } from "../../auth/useAuth"

import type { AdminUser, CreateUserRequest, UpdateUserRequest } from "../types"

export function UserManagementPanel(): React.ReactElement {
  const { users, loading, error, reload, create, update, remove } = useUsers()
  const { user: currentUser } = useAuth()

  const [createOpen, setCreateOpen] = React.useState(false)
  const [editUser, setEditUser] = React.useState<AdminUser | undefined>(
    undefined
  )
  const [deleteUser, setDeleteUser] = React.useState<AdminUser | null>(null)
  const [submitting, setSubmitting] = React.useState(false)

  async function handleCreate(input: CreateUserRequest | UpdateUserRequest) {
    setSubmitting(true)
    await create(input as CreateUserRequest)
    setSubmitting(false)
    setCreateOpen(false)
  }

  async function handleUpdate(input: CreateUserRequest | UpdateUserRequest) {
    if (!editUser) return
    setSubmitting(true)
    await update(editUser.id, input as UpdateUserRequest)
    setSubmitting(false)
    setEditUser(undefined)
  }

  async function handleDelete() {
    if (!deleteUser) return
    setSubmitting(true)
    await remove(deleteUser.id)
    setSubmitting(false)
    setDeleteUser(null)
  }

  return (
    <>
      <AdminSection
        title="User Management"
        description="Create, edit, and remove administrative users."
        actions={
          <Button onClick={() => setCreateOpen(true)}>Create User</Button>
        }
      >
        {loading && <LoadingState />}

        {error && !loading && (
          <ErrorState message={error} onRetry={reload} />
        )}

        {!loading && !error && users.length === 0 && (
          <EmptyState
            title="No users found"
            description="Create your first administrative user to get started."
            action={
              <Button onClick={() => setCreateOpen(true)}>Create User</Button>
            }
          />
        )}

        {!loading && !error && users.length > 0 && (
          <UsersTable
            users={users}
            currentUserId={currentUser?.id}
            onEdit={(u) => setEditUser(u)}
            onDelete={(u) => setDeleteUser(u)}
          />
        )}
      </AdminSection>

      <UserFormModal
        open={createOpen}
        mode="create"
        submitting={submitting}
        onSubmit={handleCreate}
        onClose={() => setCreateOpen(false)}
      />

      <UserFormModal
        open={editUser !== undefined}
        mode="edit"
        user={editUser}
        submitting={submitting}
        onSubmit={handleUpdate}
        onClose={() => setEditUser(undefined)}
      />

      <DeleteUserModal
        open={deleteUser !== null}
        user={deleteUser}
        submitting={submitting}
        onConfirm={handleDelete}
        onClose={() => setDeleteUser(null)}
      />
    </>
  )
}
