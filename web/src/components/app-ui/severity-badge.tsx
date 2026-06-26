import * as React from "react"

import { Badge } from "@/components/app-ui/badge"
import { useTranslation } from "@/lib/i18n"
import { cn } from "@/lib/utils"

type BadgeProps = Omit<React.ComponentProps<typeof Badge>, "variant">

const severityClassName: Record<string, string> = {
  critical: "border-[color-mix(in_oklch,var(--fx-severity-critical)_65%,transparent)] bg-[color-mix(in_oklch,var(--fx-severity-critical)_14%,transparent)] text-[var(--fx-severity-critical)]",
  high: "border-[color-mix(in_oklch,var(--fx-severity-high)_65%,transparent)] bg-[color-mix(in_oklch,var(--fx-severity-high)_14%,transparent)] text-[var(--fx-severity-high)]",
  medium: "border-[color-mix(in_oklch,var(--fx-severity-medium)_65%,transparent)] bg-[color-mix(in_oklch,var(--fx-severity-medium)_14%,transparent)] text-[var(--fx-severity-medium)]",
  low: "border-[color-mix(in_oklch,var(--fx-severity-low)_65%,transparent)] bg-[color-mix(in_oklch,var(--fx-severity-low)_14%,transparent)] text-[var(--fx-severity-low)]",
  none: "border-[color-mix(in_oklch,var(--fx-success)_60%,transparent)] bg-[color-mix(in_oklch,var(--fx-success)_12%,transparent)] text-[var(--fx-success)]",
}

function SeverityBadge({
  severity,
  className,
  children,
  ...props
}: { severity: string } & BadgeProps) {
  const { t } = useTranslation()
  return (
    <Badge className={cn(severityClassName[severity] ?? severityClassName.low, className)} {...props}>
      {children ?? (severity === "none" ? t("approval.clean-short") : severity)}
    </Badge>
  )
}

export { SeverityBadge }
