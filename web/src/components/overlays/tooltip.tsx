import { ReactNode } from "react";

import {
  Tooltip as ShadcnTooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

export function Tooltip({ text, children }: { text: string; children: ReactNode }) {
  return (
    <ShadcnTooltip>
      <TooltipTrigger
        className="inline-flex"
        render={<span tabIndex={0} role="img" aria-label="help" />}
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{text}</TooltipContent>
    </ShadcnTooltip>
  );
}
