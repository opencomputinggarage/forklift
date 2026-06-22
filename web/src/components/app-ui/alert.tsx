import * as React from "react"

import { cn } from "@/lib/utils"

function Alert({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      role="alert"
      data-slot="alert"
      className={cn(
        "rounded-lg border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-foreground",
        className
      )}
      {...props}
    />
  )
}

export { Alert }
