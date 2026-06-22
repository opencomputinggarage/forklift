import { CSSProperties } from "react";

import {
  Select as SelectRoot,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";

export interface SelectOption {
  value: string;
  label: string;
  description?: string;
}

export function Select({
  value,
  options,
  onChange,
  placeholder,
  style,
  size,
}: {
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  style?: CSSProperties;
  size?: "sm";
}) {
  const selected = options.find((o) => o.value === value);

  return (
    <SelectRoot value={value || null} onValueChange={(next) => onChange(next ?? "")}>
      <SelectTrigger
        size={size ?? "default"}
        style={style}
        className={cn("w-full", style?.width && "w-auto")}
      >
        <SelectValue>
          {selected ? selected.label : (
            <span className="text-muted-foreground">{placeholder ?? ""}</span>
          )}
        </SelectValue>
      </SelectTrigger>
      <SelectContent align="start">
        {options.map((o) => (
          <SelectItem key={o.value} value={o.value}>
            <span className="flex min-w-0 flex-col">
              <span>{o.label}</span>
              {o.description && (
                <span className="text-xs leading-4 text-muted-foreground">
                  {o.description}
                </span>
              )}
            </span>
          </SelectItem>
        ))}
        {options.length === 0 && (
          <div className="px-2 py-1.5 text-sm text-muted-foreground">No options</div>
        )}
      </SelectContent>
    </SelectRoot>
  );
}
