import * as React from "react"

import { Badge } from "@/components/app-ui/badge"

type BadgeProps = Omit<React.ComponentProps<typeof Badge>, "variant">

function EventBadge({ event, children, ...props }: { event: string } & BadgeProps) {
  return (
    <Badge variant="default" {...props}>
      {children ?? event}
    </Badge>
  )
}

export { EventBadge }
