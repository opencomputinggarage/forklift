import * as React from "react"

import { cn } from "@/lib/utils"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "flex field-sizing-content min-h-16 w-full rounded-lg border border-[var(--fx-border-subtle)] bg-[var(--fx-input)] px-2.5 py-2 text-base transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:shadow-[var(--fx-focus-shadow)] disabled:cursor-not-allowed disabled:bg-[var(--fx-control)] disabled:opacity-50 aria-invalid:border-destructive aria-invalid:shadow-[0_0_0_3px_color-mix(in_oklch,var(--fx-danger)_18%,transparent)] md:text-sm",
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
