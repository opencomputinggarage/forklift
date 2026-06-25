import logoUrl from "@/assets/forklift-logo.png";
import { cn } from "@/lib/utils";

// Logo renders the official forklift artwork. The source of truth is
// docs/assets/forklift-logo.png; it is copied into web/src/assets because the
// Docker web stage only ships the web/ directory.
export function Logo({ size = 34 }: { size?: number }) {
  const compact = size <= 32;
  return (
    <span
      className={cn(
        "inline-flex shrink-0 items-center justify-center overflow-hidden rounded-md bg-black",
        compact ? "size-8" : "size-[34px]"
      )}
    >
      <img src={logoUrl} alt="" className="size-full object-cover" />
    </span>
  );
}
