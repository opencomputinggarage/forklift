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
        "mb-4 flex min-w-0 items-center justify-between gap-3 max-sm:flex-col max-sm:items-stretch",
        className
      )}
    >
      <h1 className="m-0 min-w-0 text-2xl leading-tight font-semibold tracking-normal max-sm:text-xl">
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
        "mb-5 -mt-2 max-w-[820px] text-sm leading-relaxed text-muted-foreground max-sm:mb-4 max-sm:-mt-1",
        className
      )}
      {...props}
    />
  )
}

function Panel({ className, ...props }: React.ComponentProps<typeof Card>) {
  return (
    <Card
      className={cn(
        "mb-4 w-full min-w-0 max-w-full border-border/90 bg-card/95 shadow-none",
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
  return <CardContent className={cn("min-w-0 px-4 max-sm:px-3", className)} {...props} />
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
