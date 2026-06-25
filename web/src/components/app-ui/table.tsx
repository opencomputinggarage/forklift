import * as React from "react"
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table"

import {
  Table as ShadcnTable,
  TableBody as ShadcnTableBody,
  TableCell as ShadcnTableCell,
  TableHead as ShadcnTableHead,
  TableHeader as ShadcnTableHeader,
  TableRow as ShadcnTableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"

function TableWrap({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="table-wrap"
      className={cn("w-full min-w-0 max-w-full overflow-x-auto rounded-md border border-[var(--fx-border-subtle)] bg-[var(--fx-surface-panel)] overscroll-x-contain [-webkit-overflow-scrolling:touch]", className)}
      {...props}
    />
  )
}

function Table({ className, ...props }: React.ComponentProps<"table">) {
  return (
    <ShadcnTable
      data-slot="table"
      className={cn("min-w-[720px] border-collapse text-[13px] max-sm:text-sm", className)}
      {...props}
    />
  )
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return <ShadcnTableHeader data-slot="table-header" className={className} {...props} />
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return <ShadcnTableBody data-slot="table-body" className={className} {...props} />
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <ShadcnTableRow
      data-slot="table-row"
      className={cn(
        "border-[var(--fx-border-subtle)] last:border-0 hover:bg-[var(--fx-surface-hover)] data-[state=selected]:bg-[var(--fx-surface-selected)]",
        className
      )}
      {...props}
    />
  )
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <ShadcnTableHead
      data-slot="table-head"
      className={cn(
        "h-auto border-b border-[var(--fx-border-subtle)] bg-[var(--fx-surface-panel-raised)] px-2.5 py-2 text-[11px] font-medium text-[var(--fx-text-subtle)] uppercase",
        className
      )}
      {...props}
    />
  )
}

function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <ShadcnTableCell
      data-slot="table-cell"
      className={cn("px-2.5 py-2 align-middle", className)}
      {...props}
    />
  )
}

function DataTable<TData>({
  columns,
  data,
  empty,
  className,
  tableClassName,
}: {
  columns: ColumnDef<TData>[]
  data: TData[]
  empty?: React.ReactNode
  className?: string
  tableClassName?: string
}) {
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  const columnCount = table.getAllLeafColumns().length

  return (
    <TableWrap className={className}>
      <Table className={tableClassName}>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <TableHead key={header.id}>
                  {header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.length > 0 ? (
            table.getRowModel().rows.map((row) => (
              <TableRow key={row.id}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell colSpan={columnCount} className="text-muted-foreground">
                {empty ?? "No results."}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </TableWrap>
  )
}

export {
  DataTable,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrap,
  type ColumnDef,
}
