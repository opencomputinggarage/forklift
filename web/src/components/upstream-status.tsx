import { useEffect, useState } from "react";
import { api, UpstreamHealth } from "../api";
import { Button } from "@/components/ui/button";

// UpstreamStatus probes a proxy repository's upstream and renders a health
// badge. compact shows only reachable/unreachable (list view); the full form
// (detail view) also shows the status code and latency. withButton adds a
// "Recheck" action.
export function UpstreamStatus({
  repoId,
  withButton,
  compact,
}: {
  repoId: number;
  withButton?: boolean;
  compact?: boolean;
}) {
  const [h, setH] = useState<UpstreamHealth | null>(null);
  const [loading, setLoading] = useState(true);

  const check = () => {
    setLoading(true);
    api
      .upstreamHealth(repoId)
      .then(setH)
      .catch(() => setH({ applicable: true, reachable: false, error: "check failed" }))
      .finally(() => setLoading(false));
  };
  useEffect(check, [repoId]);

  let badge;
  if (loading) badge = <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><span className="inline-block size-[9px] rounded-full border border-muted-foreground bg-transparent" /> checking…</span>;
  else if (!h || !h.applicable) badge = <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">—</span>;
  else if (h.reachable)
    badge = (
      <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
        <span className="inline-block size-[9px] rounded-full border border-emerald-400 bg-emerald-400 shadow-[0_0_8px_color-mix(in_oklch,var(--success)_60%,transparent)]" /> reachable{!compact && <> · {h.status} · {h.latency_ms}ms</>}
      </span>
    );
  else
    badge = <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground" title={h.error}><span className="inline-block size-[9px] rounded-full border border-red-500 bg-red-500 shadow-[0_0_5px_rgba(239,68,68,0.7)]" /> unreachable</span>;

  if (!withButton) return badge;
  return (
    <span className="flex items-center gap-2.5 max-[760px]:flex-col max-[760px]:items-stretch" style={{ gap: 10 }}>
      {badge}
      <Button variant="outline" type="button" onClick={check} disabled={loading}>Recheck</Button>
    </span>
  );
}
