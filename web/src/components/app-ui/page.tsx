import * as React from "react"

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

export { PageDescription, PageHeader }
