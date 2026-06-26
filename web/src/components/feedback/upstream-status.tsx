import { useEffect, useState } from "react";
import { api, UpstreamHealth } from "@/api";
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

  // signal is supplied by the mount effect so an in-flight probe is aborted when
  // repoId changes or the component unmounts; results from an aborted (stale)
  // request are ignored. The Recheck button calls check() with no signal.
  const check = (signal?: AbortSignal) => {
    setLoading(true);
    api
      .upstreamHealth(repoId, signal)
      .then((res) => {
        if (signal?.aborted) return;
        setH(res);
        setLoading(false);
      })
      .catch(() => {
        if (signal?.aborted) return;
        setH({ applicable: true, reachable: false, error: "check failed" });
        setLoading(false);
      });
  };
  useEffect(() => {
    const ctl = new AbortController();
    check(ctl.signal);
    return () => ctl.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repoId]);

  let badge;
  if (loading) badge = <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><span className="inline-block size-[9px] rounded-full border border-muted-foreground bg-transparent" /> checking…</span>;
  else if (!h || !h.applicable) badge = <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">—</span>;
  else if (h.reachable)
    badge = (
      <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
        <span className="inline-block size-[9px] rounded-full border border-[var(--fx-success)] bg-[var(--fx-success)]" /> reachable{!compact && <> · {h.status} · {h.latency_ms}ms</>}
      </span>
    );
  else
    badge = <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground" title={h.error}><span className="inline-block size-[9px] rounded-full border border-[var(--fx-danger)] bg-[var(--fx-danger)]" /> unreachable</span>;

  if (!withButton) return badge;
  return (
    <span className="flex items-center gap-2.5 max-sm:flex-col max-sm:items-stretch">
      {badge}
      <Button variant="outline" type="button" onClick={() => check()} disabled={loading}>Recheck</Button>
    </span>
  );
}
