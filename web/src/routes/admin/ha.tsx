import { useEffect, useRef, useState } from "react";
import { createFileRoute, Navigate } from "@tanstack/react-router";
import { api, type HAStatus as HAStatusType } from "@/api";
import { useAuth } from "@/authContext";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
  TableWrap,
} from "@/components/app-ui/table";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/admin/ha")({
  component: AdminHARoute,
});

function AdminHARoute() {
  const { me } = useAuth();
  if (!me.admin) return <Navigate to="/workspace/repositories" replace />;

  return (
    <>
      <PageHeader title="HA Status" />
      <HAStatusPanel />
    </>
  );
}

// How often the HA status is re-fetched; the header shows a live countdown to
// the next refresh so a failover (leader change) is visible without a manual
// refresh.
const HA_REFRESH_MS = 5_000;

// formatUptime renders the elapsed time since startedAt as "Xd Yh Zm Ws",
// dropping leading zero units. Recomputed on each render (the countdown tick) so
// it counts up live.
function formatUptime(startedAt: string): string {
  const ms = Date.now() - new Date(startedAt).getTime();
  if (!isFinite(ms) || ms < 0) return "—";
  const s = Math.floor(ms / 1000);
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60), sec = s % 60;
  const parts: string[] = [];
  if (d) parts.push(`${d}d`);
  if (h || d) parts.push(`${h}h`);
  if (m || h || d) parts.push(`${m}m`);
  parts.push(`${sec}s`);
  return parts.join(" ");
}

// HAStatusPanel renders the live HA cluster topology and status, plus the
// manual-failover (step-down) control. Embedded as the "HA Status" tab on the
// admin page; the step-down danger zone only shows for the active leader.
export function HAStatusPanel() {
  const [status, setStatus] = useState<HAStatusType | null>(null);
  const [error, setError] = useState("");
  const [confirmStepDown, setConfirmStepDown] = useState(false);
  const [stepping, setStepping] = useState(false);
  const [notice, setNotice] = useState("");
  const [secsLeft, setSecsLeft] = useState(HA_REFRESH_MS / 1000);
  const nextAt = useRef(0);

  const load = () =>
    api.getHA()
      .then((next) => { setStatus(next); setError(""); })
      .catch((err) => setError((err as Error).message));

  // reload fetches and restarts the countdown to the next auto-refresh.
  const reload = () => {
    nextAt.current = Date.now() + HA_REFRESH_MS;
    setSecsLeft(HA_REFRESH_MS / 1000);
    return load();
  };

  useEffect(() => {
    reload();
    // Tick a few times a second so the countdown updates smoothly, and fire the
    // next fetch when it reaches zero.
    const timer = window.setInterval(() => {
      const remMs = nextAt.current - Date.now();
      setSecsLeft(Math.max(0, Math.ceil(remMs / 1000)));
      if (remMs <= 0) reload();
    }, 250);
    return () => window.clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const stepDown = async () => {
    setConfirmStepDown(false);
    setStepping(true);
    setError("");
    setNotice("");
    try {
      await api.stepDownHA();
      setNotice("Stepping down — a standby is taking over. Traffic follows the new leader.");
      reload();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setStepping(false);
    }
  };

  // Manual failover only makes sense in HA mode when this pod is the active
  // leader; a standby has nothing to release.
  const canStepDown = Boolean(status?.enabled && status?.is_leader);

  return (
    <>
      <PageDescription>
        Review storage topology, leader election state, and fencing token for the active cluster.
      </PageDescription>

      <Card size="sm" className="mb-4">
        <CardContent>
          <div className="flex min-w-0 items-center gap-2 mb-4 max-sm:flex-wrap justify-between gap-3 max-sm:flex-col max-sm:items-start">
            <h2 className="m-0 text-base font-semibold">Cluster status</h2>
            <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap gap-2 max-sm:w-full max-sm:justify-between">
              <span
                className="inline-flex items-center gap-1.5 rounded-full border border-border bg-input px-[9px] py-0.5 text-[11px] text-muted-foreground tabular-nums"
                title="Auto-refreshes the HA status"
              >
                <span
                  className="size-1.5 flex-none rounded-full bg-[var(--fx-success)] [animation:refresh-pulse_1.4s_ease-in-out_infinite] motion-reduce:animate-none"
                  aria-hidden="true"
                />
                auto-refresh {secsLeft}s
              </span>
              <Button variant="outline" type="button" onClick={reload}>
                Refresh
              </Button>
            </div>
          </div>

          {error && <Alert className="mb-4">{error}</Alert>}
          {notice && <div className="mb-4 text-sm text-muted-foreground">{notice}</div>}
          {!status ? (
            <p className="m-0 text-sm text-muted-foreground">Loading...</p>
          ) : (
            <>
              <HAArchitecture status={status} />
              <TableWrap className="mt-4">
                <Table>
                  <TableBody>
                    <TableRow><TableCell className="w-44 text-muted-foreground">Mode</TableCell><TableCell>{status.mode}</TableCell></TableRow>
                    <TableRow><TableCell className="text-muted-foreground">Storage backend</TableCell><TableCell>{status.backend}</TableCell></TableRow>
                    <TableRow><TableCell className="text-muted-foreground">Leader election</TableCell><TableCell>{status.enabled ? "enabled" : "disabled (single instance)"}</TableCell></TableRow>
                    <TableRow><TableCell className="text-muted-foreground">This pod</TableCell><TableCell className="font-mono text-xs">{status.identity || "-"}</TableCell></TableRow>
                    <TableRow>
                      <TableCell className="text-muted-foreground">Role</TableCell>
                      <TableCell>
                        <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap gap-2">
                          <Badge variant={status.is_leader ? "success" : "outline"}>
                            {status.role || "-"}
                          </Badge>
                          {status.is_leader && <span className="text-sm text-muted-foreground">serving traffic</span>}
                        </div>
                      </TableCell>
                    </TableRow>
                    <TableRow><TableCell className="text-muted-foreground">Current leader</TableCell><TableCell className="font-mono text-xs">{status.leader || "-"}</TableCell></TableRow>
                    {status.version && <TableRow><TableCell className="text-muted-foreground">Version</TableCell><TableCell>{status.version}</TableCell></TableRow>}
                    {status.started_at && (
                      <TableRow>
                        <TableCell className="text-muted-foreground">Uptime</TableCell>
                        <TableCell>{formatUptime(status.started_at)} <span className="text-muted-foreground">· since {status.started_at.slice(0, 19).replace("T", " ")}</span></TableCell>
                      </TableRow>
                    )}
                    {status.lease_name && <TableRow><TableCell className="text-muted-foreground">Lease</TableCell><TableCell className="font-mono text-xs">{status.lease_name}</TableCell></TableRow>}
                    {typeof status.fencing_token === "number" && status.fencing_token > 0 && (
                      <TableRow><TableCell className="text-muted-foreground">Fencing token</TableCell><TableCell>{status.fencing_token}</TableCell></TableRow>
                    )}
                  </TableBody>
                </Table>
              </TableWrap>
            </>
          )}

          <p className="mb-0 mt-4 text-sm leading-relaxed text-muted-foreground">
            In HA only the leader serves traffic; standby pods stay ready and take over automatically on failover.
            The fencing token guards object-storage metadata against a superseded leader.
          </p>
        </CardContent>
      </Card>

      {canStepDown && (
        <Card size="sm" className="mb-4 border-destructive/70">
          <CardContent>
            <h2 className="m-0 mb-3 text-base font-semibold text-destructive">Danger zone</h2>
            <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap justify-between gap-4 max-sm:flex-col max-sm:items-start">
              <p className="m-0 text-sm leading-relaxed text-muted-foreground">
                Manual failover gracefully steps this leader down so a standby is elected and takes over.
                The leader releases its lease and traffic moves to the new leader. This pod stays Ready and can
                be re-elected later. Use it for planned maintenance or to drain this pod. Expect a brief
                in-flight disruption while traffic moves.
              </p>
              <Button variant="destructive" type="button" disabled={stepping}
                onClick={() => setConfirmStepDown(true)}>
                {stepping ? "Stepping down…" : "Step down (manual failover)"}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      <ConfirmModal
        open={confirmStepDown}
        title="Step down as leader?"
        message="This leader gracefully releases its lease and a standby is elected. Traffic moves to the new leader. Expect a brief disruption. This pod stays Ready and can be re-elected later."
        confirmLabel="Step down"
        danger
        onConfirm={stepDown}
        onCancel={() => setConfirmStepDown(false)}
      />
    </>
  );
}

// HAArchitecture renders the live active/standby topology as an SVG: client →
// Service → the active leader pod, the standby kept Ready alongside, the election
// Lease linking the two, and the shared storage the single writer owns. It tracks
// the live status, so on a failover the ACTIVE/STANDBY roles swap on the next
// poll. In single-instance mode (election disabled) only the lone active pod is
// drawn. Active elements are green; the standby and its paths are dashed/dimmed.
function HAArchitecture({ status }: { status: HAStatusType }) {
  const ha = status.enabled;
  const s3 = status.backend === "s3";
  const showFencing = s3 && typeof status.fencing_token === "number" && status.fencing_token > 0;
  const trunc = (s: string, n = 20) => (s.length > n ? s.slice(0, n - 1) + "…" : s);

  const activeName = status.leader || status.identity || "—";
  const activeThis = status.is_leader;
  const standbyName = status.is_leader ? "standby peer" : status.identity || "—";
  const standbyThis = !status.is_leader;
  // Pod sub-line: this pod's forklift version, plus a "this pod" marker on the
  // box we are connected to. (Only this pod's version is known to the API.)
  const ver = status.version ? (/^\d/.test(status.version) ? `v${status.version}` : status.version) : "";
  const podSub = (thisPod: boolean) => [ver, thisPod ? "this pod" : ""].filter(Boolean).join(" · ");
  // Active pod sits at the top when a standby is drawn below it; centred when
  // alone (single instance). The whole flow reads left to right.
  const apY = ha ? 38 : 122;
  const apCy = apY + 46;
  const boxClass = "fill-[var(--input-bg)] stroke-border stroke-[1.5]";
  const activeBoxClass = "stroke-[var(--fx-success)]";
  const standbyBoxClass = "[stroke-dasharray:6_4]";
  const edgeClass = "fill-none stroke-muted-foreground stroke-[1.5]";
  const activeEdgeClass = "stroke-[var(--fx-success)]";
  const dimEdgeClass = "opacity-50 [stroke-dasharray:6_4]";
  const flowEdgeClass = "[stroke-dasharray:7_5] [animation:ha-edge-flow_0.7s_linear_infinite] motion-reduce:animate-none motion-reduce:[stroke-dasharray:none]";
  const labelClass = "fill-foreground text-[13px]";
  const monoLabelClass = "fill-foreground font-mono text-xs";
  const subLabelClass = "fill-muted-foreground text-[11px]";
  const tagClass = "text-[11px] tracking-[0.06em]";

  return (
    <div className="mt-3 w-full overflow-x-auto">
      <div className="mb-1.5 text-xs text-muted-foreground">Active / standby topology</div>
      <svg className="block h-auto w-full min-w-[720px]" viewBox="0 0 1080 340" role="img"
        aria-label="High availability active/standby architecture">
        <defs>
          <marker id="ha-head" viewBox="0 0 8 8" markerWidth="7" markerHeight="7" refX="6.5" refY="4" orient="auto-start-reverse">
            <path className="fill-muted-foreground" d="M0,1 L7,4 L0,7 Z" />
          </marker>
          <marker id="ha-head-ok" viewBox="0 0 8 8" markerWidth="7" markerHeight="7" refX="6.5" refY="4" orient="auto-start-reverse">
            <path className="fill-[var(--fx-success)]" d="M0,1 L7,4 L0,7 Z" />
          </marker>
        </defs>

        {/* Client */}
        <rect className={boxClass} x="20" y="140" width="150" height="56" rx="8" />
        <text className={labelClass} x="95" y="164" textAnchor="middle">Client</text>
        <text className={subLabelClass} x="95" y="181" textAnchor="middle">package managers</text>

        {/* Service */}
        <rect className={boxClass} x="250" y="134" width="180" height="68" rx="8" />
        <text className={labelClass} x="340" y="164" textAnchor="middle">Service (forklift)</text>
        <text className={subLabelClass} x="340" y="182" textAnchor="middle">routes to leader</text>

        {/* Client -> Service */}
        <line className={cn(edgeClass, flowEdgeClass)} x1="170" y1="168" x2="250" y2="168" markerEnd="url(#ha-head)" />
        <text className={subLabelClass} x="210" y="160" textAnchor="middle">HTTP</text>

        {/* Active pod — crowned, since this box is always the active leader. */}
        <rect className={cn(boxClass, activeBoxClass)} x="540" y={apY} width="230" height="92" rx="8" />
        <path className="fill-[var(--fx-success)]"
          d={`M644,${apY + 17} L644,${apY + 7} L649.5,${apY + 12} L655,${apY + 5} L660.5,${apY + 12} L666,${apY + 7} L666,${apY + 17} Z`} />
        <text className={cn(tagClass, "fill-[var(--fx-success)]")} x="655" y={apY + 33} textAnchor="middle">
          {ha ? "ACTIVE · LEADER" : "ACTIVE · SINGLE INSTANCE"}
        </text>
        <text className={monoLabelClass} x="655" y={apY + 52} textAnchor="middle"><title>{activeName}</title>{trunc(activeName, 24)}</text>
        {podSub(activeThis) && <text className={subLabelClass} x="655" y={apY + 72} textAnchor="middle">{podSub(activeThis)}</text>}

        {/* Service -> Active (active path) */}
        <line className={cn(edgeClass, activeEdgeClass, flowEdgeClass)} x1="430" y1={ha ? 150 : 168} x2="540" y2={apCy} markerEnd="url(#ha-head-ok)" />

        {/* Active -> Storage (single writer) */}
        <line className={cn(edgeClass, activeEdgeClass, flowEdgeClass)} x1="770" y1={apCy} x2="900" y2={ha ? 150 : 168} markerEnd="url(#ha-head-ok)" />
        <text className={subLabelClass} x="835" y={ha ? 108 : 160} textAnchor="middle">single writer</text>

        {ha && (
          <>
            {/* Standby pod */}
            <rect className={cn(boxClass, standbyBoxClass)} x="540" y="212" width="230" height="92" rx="8" />
            <text className={cn(tagClass, "fill-muted-foreground")} x="655" y="240" textAnchor="middle">STANDBY</text>
            <text className={monoLabelClass} x="655" y="264" textAnchor="middle"><title>{standbyName}</title>{trunc(standbyName, 24)}</text>
            {podSub(standbyThis) && <text className={subLabelClass} x="655" y="284" textAnchor="middle">{podSub(standbyThis)}</text>}

            {/* Service -> Standby (ready, not served) */}
            <line className={cn(edgeClass, dimEdgeClass)} x1="430" y1="186" x2="540" y2="258" markerEnd="url(#ha-head)" />
            <text className={subLabelClass} x="485" y="232" textAnchor="middle">ready</text>

            {/* Lease link between the two pods (vertical) */}
            <line className={edgeClass} x1="655" y1="132" x2="655" y2="210" markerStart="url(#ha-head)" markerEnd="url(#ha-head)" />
            <text className={subLabelClass} x="678" y="166" textAnchor="start">Lease</text>
            <text className={subLabelClass} x="678" y="182" textAnchor="start">leader election</text>

            {/* Standby -> Storage */}
            <line className={cn(edgeClass, dimEdgeClass)} x1="770" y1="258" x2="900" y2="186" markerEnd="url(#ha-head)" />
            <text className={subLabelClass} x="835" y="232" textAnchor="middle">{s3 ? "syncs" : "standby"}</text>
          </>
        )}

        {/* Storage */}
        <rect className={boxClass} x="900" y="124" width="170" height="88" rx="8" />
        <text className={labelClass} x="985" y="158" textAnchor="middle">{s3 ? "Object Storage" : "Block Storage"}</text>
        <text className={cn(subLabelClass, "font-mono")} x="985" y="177" textAnchor="middle">
          <title>{status.storage_endpoint || "—"}</title>{trunc(status.storage_endpoint || "—", 24)}
        </text>
        <text className={subLabelClass} x="985" y="194" textAnchor="middle">
          {s3 ? (showFencing ? `fenced · token ${status.fencing_token}` : "fenced writes") : "single writer"}
        </text>
      </svg>
    </div>
  );
}
