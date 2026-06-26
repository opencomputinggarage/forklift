import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"

import { Badge as ShadcnBadge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex w-fit shrink-0 items-center rounded-md border px-1.5 py-0.5 text-xs leading-normal font-medium whitespace-nowrap",
  {
    variants: {
      variant: {
        default: "border-[var(--fx-border-subtle)] bg-[var(--fx-control)] text-muted-foreground",
        secondary: "border-[var(--fx-border-subtle)] bg-[var(--fx-surface-panel-raised)] text-secondary-foreground",
        outline: "border-border bg-transparent text-muted-foreground",
        success: "border-[color-mix(in_oklch,var(--fx-success)_58%,transparent)] bg-[color-mix(in_oklch,var(--fx-success)_12%,transparent)] text-[var(--fx-success)]",
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

export { Badge }
