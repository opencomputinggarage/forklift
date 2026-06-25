import * as React from "react"

import { Badge } from "@/components/app-ui/badge"
import { cn } from "@/lib/utils"

type BadgeProps = Omit<React.ComponentProps<typeof Badge>, "variant">

const severityClassName: Record<string, string> = {
  critical: "border-red-500/70 bg-red-500 text-white",
  high: "border-orange-500/70 bg-orange-500 text-white",
  medium: "border-amber-500/70 bg-amber-500 text-black",
  low: "border-slate-400/70 bg-slate-400 text-black",
  none: "border-emerald-500/70 bg-emerald-600 text-white",
}

function SeverityBadge({
  severity,
  className,
  children,
  ...props
}: { severity: string } & BadgeProps) {
  return (
    <Badge className={cn(severityClassName[severity] ?? severityClassName.low, className)} {...props}>
      {children ?? (severity === "none" ? "clean" : severity)}
    </Badge>
  )
}

export { SeverityBadge }
