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
        "mb-4 flex items-center justify-between gap-3 max-sm:flex-col max-sm:items-start",
        className
      )}
    >
      <h1 className="m-0 text-2xl leading-tight font-semibold tracking-normal">
        {title}
      </h1>
      {actions}
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
        "mb-5 -mt-2 max-w-[820px] text-sm leading-relaxed text-muted-foreground",
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
        "mb-4 border-border/90 bg-card/95 shadow-none",
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
  return <CardContent className={cn("px-4", className)} {...props} />
}

function Inline({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn("flex items-center gap-2", className)}
      {...props}
    />
  )
}

export { Inline, PageDescription, PageHeader, Panel, PanelBody }
