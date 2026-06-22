import * as React from "react"

import { Badge } from "@/components/app-ui/badge"
import { cn } from "@/lib/utils"

type BadgeProps = Omit<React.ComponentProps<typeof Badge>, "variant">

function ActionBadge({ action, children, ...props }: { action: string } & BadgeProps) {
  return (
    <Badge variant="default" {...props}>
      {children ?? action}
    </Badge>
  )
}

function PermissionBadge({ className, ...props }: BadgeProps) {
  return <Badge variant="default" className={cn("font-mono", className)} {...props} />
}

function RoleBadge(props: BadgeProps) {
  return <Badge variant="default" {...props} />
}

export { ActionBadge, PermissionBadge, RoleBadge }
