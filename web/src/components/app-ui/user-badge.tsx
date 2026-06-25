import * as React from "react"

import { Badge } from "@/components/app-ui/badge"

type BadgeProps = Omit<React.ComponentProps<typeof Badge>, "variant">

function UserBadge({ username, children, ...props }: { username: string } & BadgeProps) {
  return (
    <Badge variant="default" {...props}>
      {children ?? username}
    </Badge>
  )
}

export { UserBadge }
