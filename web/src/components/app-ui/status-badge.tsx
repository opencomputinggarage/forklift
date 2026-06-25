import * as React from "react"

import { Badge } from "@/components/app-ui/badge"
import { cn } from "@/lib/utils"

type BadgeProps = Omit<React.ComponentProps<typeof Badge>, "variant">

const approvalVariant: Record<string, React.ComponentProps<typeof Badge>["variant"]> = {
  pending: "warning",
  approved: "success",
  rejected: "destructive",
}

const stateVariant: Record<string, React.ComponentProps<typeof Badge>["variant"]> = {
  active: "success",
  online: "success",
  approved: "success",
  disabled: "destructive",
  locked: "destructive",
  offline: "destructive",
  rejected: "destructive",
  pending: "warning",
}

function ApprovalStatusBadge({
  status,
  children,
  ...props
}: { status: string } & BadgeProps) {
  return (
    <Badge variant={approvalVariant[status] ?? "default"} {...props}>
      {children ?? status}
    </Badge>
  )
}

function StateBadge({
  state,
  children,
  ...props
}: { state: string } & BadgeProps) {
  return (
    <Badge variant={stateVariant[state] ?? "default"} {...props}>
      {children ?? state}
    </Badge>
  )
}

function CountBadge({ className, ...props }: BadgeProps) {
  return <Badge variant="default" className={cn("tabular-nums", className)} {...props} />
}

export { ApprovalStatusBadge, CountBadge, StateBadge }
