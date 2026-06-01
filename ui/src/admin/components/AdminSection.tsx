import * as React from "react"

import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
} from "@/components/ui/card"

interface AdminSectionProps {
  title: string
  description?: string
  actions?: React.ReactNode
  children: React.ReactNode
}

export function AdminSection({
  title,
  description,
  actions,
  children,
}: AdminSectionProps): React.ReactElement {
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h2>{title}</h2>
        </CardTitle>
        {description && (
          <CardDescription>{description}</CardDescription>
        )}
        {actions && <CardAction>{actions}</CardAction>}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  )
}
