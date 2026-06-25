import logoUrl from "@/assets/forklift-logo.svg";
import { cn } from "@/lib/utils";

// Logo renders the official forklift artwork. The source of truth is
// docs/assets/forklift-logo.svg; it is copied into web/src/assets because the
// Docker web stage only ships the web/ directory. The artwork is black ink on
// transparent, so it sits on a light rounded chip to stay readable on the
// dark sidebar.
export function Logo({ size = 34 }: { size?: number }) {
  const compact = size <= 32;
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded-md bg-[#f2f3f5]",
        compact ? "size-8" : "size-[34px]"
      )}
    >
      <img src={logoUrl} alt="" className={compact ? "size-[26px]" : "size-[27px]"} />
    </span>
  );
}
