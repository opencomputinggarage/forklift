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
  className,
  size,
}: {
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
  size?: "sm";
}) {
  const hasEmptyOption = options.some((o) => o.value === "");
  const selectValue = value === "" && !hasEmptyOption ? null : value;

  return (
    <SelectRoot
      items={options}
      value={selectValue}
      onValueChange={(next) => onChange(next ?? "")}
    >
      <SelectTrigger
        size={size ?? "default"}
        className={cn("w-full", className)}
      >
        <SelectValue placeholder={placeholder ?? ""} />
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
