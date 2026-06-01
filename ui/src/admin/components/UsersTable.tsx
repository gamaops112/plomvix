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

import type { AdminUser } from "../types"
import { formatDateTime, titleCase } from "../format"

interface UsersTableProps {
  users: AdminUser[]
  currentUserId?: string
  onEdit: (user: AdminUser) => void
  onDelete: (user: AdminUser) => void
}

export function UsersTable({
  users,
  currentUserId,
  onEdit,
  onDelete,
}: UsersTableProps): React.ReactElement | null {
  if (users.length === 0) return null

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Username</TableHead>
          <TableHead>Role</TableHead>
          <TableHead>Created At</TableHead>
          <TableHead>Updated At</TableHead>
          <TableHead className="w-0">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {users.map((u) => {
          const isSelf = u.id === currentUserId
          return (
            <TableRow key={u.id} className={isSelf ? "border-l-2 border-l-primary/40" : ""}>
              <TableCell>
                <span className="flex items-center gap-2">
                  {u.username}
                  {isSelf && (
                    <Badge variant="outline" className="text-xs font-normal">
                      you
                    </Badge>
                  )}
                </span>
              </TableCell>
              <TableCell>
                <Badge>{titleCase(u.role)}</Badge>
              </TableCell>
              <TableCell>{formatDateTime(u.created_at)}</TableCell>
              <TableCell>{formatDateTime(u.updated_at)}</TableCell>
              <TableCell>
                <div className="flex items-center gap-1">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => onEdit(u)}
                  >
                    Edit
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    disabled={isSelf}
                    onClick={() => onDelete(u)}
                  >
                    Delete
                  </Button>
                </div>
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}
