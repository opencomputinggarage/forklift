import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"

import { Badge as ShadcnBadge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex w-fit shrink-0 items-center rounded-full border px-2 py-0.5 text-xs leading-normal font-medium whitespace-nowrap",
  {
    variants: {
      variant: {
        default: "border-border bg-muted text-muted-foreground",
        secondary: "border-border bg-secondary text-secondary-foreground",
        outline: "border-border bg-transparent text-muted-foreground",
        success: "border-emerald-400/60 bg-emerald-400/10 text-emerald-300",
        warning: "border-primary/70 bg-primary/10 text-primary",
        destructive: "border-destructive/70 bg-destructive/10 text-destructive",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function Badge({
  className,
  variant,
  ...props
}: Omit<React.ComponentProps<typeof ShadcnBadge>, "variant"> & VariantProps<typeof badgeVariants>) {
  return (
    <ShadcnBadge
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  )
}

export { Badge, badgeVariants }
