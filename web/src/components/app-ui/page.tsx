import * as React from "react"

import { Card, CardContent } from "@/components/ui/card"
import { cn } from "@/lib/utils"

function PageHeader({
  title,
  actions,
  className,
}: {
  title: React.ReactNode
  actions?: React.ReactNode
  className?: string
}) {
  return (
    <div
      className={cn(
        "mb-3 flex min-w-0 items-end justify-between gap-3 border-b border-[var(--fx-border-subtle)] pb-3 max-sm:flex-col max-sm:items-stretch",
        className
      )}
    >
      <h1 className="m-0 min-w-0 text-[1.35rem] leading-tight font-semibold tracking-normal max-sm:text-lg">
        {title}
      </h1>
      {actions && (
        <div className="flex shrink-0 items-center gap-2 max-sm:w-full max-sm:flex-wrap max-sm:[&>*]:min-w-0 max-sm:[&>*]:flex-1">
          {actions}
        </div>
      )}
    </div>
  )
}

function PageDescription({
  className,
  ...props
}: React.ComponentProps<"p">) {
  return (
    <p
      className={cn(
        "mb-5 -mt-1 max-w-[820px] text-sm leading-6 text-muted-foreground max-sm:mb-4 max-sm:mt-0",
        className
      )}
      {...props}
    />
  )
}

function Panel({ className, ...props }: React.ComponentProps<typeof Card>) {
  return (
    <Card
      size="sm"
      className={cn(
        "mb-4 w-full min-w-0 max-w-full bg-[var(--fx-surface-panel)] shadow-[var(--fx-panel-highlight)]",
        className
      )}
      {...props}
    />
  )
}

function PanelBody({
  className,
  ...props
}: React.ComponentProps<typeof CardContent>) {
  return <CardContent className={cn("min-w-0 px-3 max-sm:px-2.5", className)} {...props} />
}

function Inline({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn("flex min-w-0 items-center gap-2 max-sm:flex-wrap", className)}
      {...props}
    />
  )
}

export { Inline, PageDescription, PageHeader, Panel, PanelBody }
