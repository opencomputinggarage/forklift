import * as React from "react"

import { Badge } from "@/components/app-ui/badge"

type BadgeProps = Omit<React.ComponentProps<typeof Badge>, "variant">

function SourceBadge({ source, children, ...props }: { source: string } & BadgeProps) {
  return (
    <Badge variant="default" {...props}>
      {children ?? source}
    </Badge>
  )
}

export { SourceBadge }
