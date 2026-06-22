import * as React from "react"

import { Alert as ShadcnAlert } from "@/components/ui/alert"
import { cn } from "@/lib/utils"

function Alert({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <ShadcnAlert
      variant="destructive"
      className={cn(
        "border-destructive/50 bg-destructive/10 text-foreground",
        className
      )}
      {...props}
    />
  )
}

export { Alert }
