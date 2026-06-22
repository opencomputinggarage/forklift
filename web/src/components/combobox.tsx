import { CSSProperties, KeyboardEvent, useEffect, useRef, useState } from "react";

import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

// Combobox is an editable autocomplete input: it accepts free text (so wildcard
// patterns like `*` or `maven-*` still work) while offering a filtered dropdown
// of known values.
export function Combobox({ value, onChange, options, placeholder, style, hints }: {
  value: string;
  onChange: (value: string) => void;
  options: string[];
  placeholder?: string;
  style?: CSSProperties;
  // hints maps an option to a muted secondary label (e.g. a repository type)
  // shown in the dropdown; the picked value is still the option string itself.
  hints?: Record<string, string>;
}) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  const rootRef = useRef<HTMLDivElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  // Substring match, case-insensitive. An empty query lists every option.
  const q = value.trim().toLowerCase();
  const matches = options.filter((o) => o.toLowerCase().includes(q));

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [open]);

  // Keep the highlighted option in view while navigating with the keyboard.
  useEffect(() => {
    if (open && active >= 0)
      menuRef.current?.children[active]?.scrollIntoView({ block: "nearest" });
  }, [open, active]);

  const pick = (v: string) => {
    onChange(v);
    setOpen(false);
    setActive(-1);
  };

  const onKeyDown = (e: KeyboardEvent) => {
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setOpen(true);
        setActive((i) => Math.min(matches.length - 1, i + 1));
        break;
      case "ArrowUp":
        e.preventDefault();
        setActive((i) => Math.max(0, i - 1));
        break;
      case "Enter":
        // Only intercept Enter to accept a highlighted suggestion; otherwise let
        // the keystroke through (free-typed patterns submit the form normally).
        if (open && active >= 0) {
          e.preventDefault();
          pick(matches[active]);
        }
        break;
      case "Escape":
        setOpen(false);
        break;
    }
  };

  return (
    <div ref={rootRef} className="relative" style={style}>
      <Input value={value} placeholder={placeholder}
        role="combobox" aria-expanded={open} aria-autocomplete="list"
        onChange={(e) => { onChange(e.target.value); setOpen(true); setActive(-1); }}
        onFocus={() => setOpen(true)}
        onKeyDown={onKeyDown} />
      {open && matches.length > 0 && (
        <div
          ref={menuRef}
          className="absolute z-50 mt-1 max-h-64 w-full min-w-40 overflow-y-auto rounded-lg bg-popover p-1 text-popover-foreground shadow-md ring-1 ring-foreground/10"
          role="listbox"
        >
          {matches.map((o, i) => (
            <div key={o} role="option" aria-selected={o === value}
              className={cn(
                "flex cursor-default items-center justify-between gap-2 rounded-md px-1.5 py-1 text-sm outline-hidden select-none",
                i === active && "bg-accent text-accent-foreground",
                o === value && "text-primary"
              )}
              onMouseEnter={() => setActive(i)}
              // mousedown (not click) so the option is picked before the input blurs.
              onMouseDown={(e) => { e.preventDefault(); pick(o); }}>
              <span className="min-w-0 truncate">
                {o}
                {hints?.[o] && (
                  <span className="ml-2 text-xs text-muted-foreground">
                    {hints[o]}
                  </span>
                )}
              </span>
              {o === value && <span>✓</span>}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
