import * as React from "react"

import { Badge } from "@/components/app-ui/badge"

type BadgeProps = Omit<React.ComponentProps<typeof Badge>, "variant">

function RepoTypeBadge({ type, children, ...props }: { type: string } & BadgeProps) {
  return (
    <Badge variant="secondary" {...props}>
      {children ?? type}
    </Badge>
  )
}

function FormatBadge({ format, children, ...props }: { format: string } & BadgeProps) {
  return (
    <Badge variant="default" {...props}>
      {children ?? format}
    </Badge>
  )
}

export { FormatBadge, RepoTypeBadge }
