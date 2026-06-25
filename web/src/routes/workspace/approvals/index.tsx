import { FormEvent, ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import { createFileRoute, Link, Navigate } from "@tanstack/react-router";
import { Approval, Repository, VersionDeny, api } from "@/api";
import { useAuth } from "@/authContext";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { Alert } from "@/components/app-ui/alert";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Select } from "@/components/app-ui/select";
import { ApprovalStatusBadge } from "@/components/app-ui/status-badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrap,
} from "@/components/app-ui/table";
import { Button } from "@/components/ui/button";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SeverityBadge } from "@/components/app-ui/severity-badge";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/workspace/approvals/")({
  component: ApprovalsRoute,
});

function ApprovalsRoute() {
  const { me } = useAuth();
  return me.admin || me.approver || me.auditor ? <Approvals /> : <Navigate to="/workspace/repositories" replace />;
}

// ApprovalVulnBadge shows the OSV scan result so a reviewer sees known
// advisories before approving. A scanned coordinate with no advisories shows a
// green "clean" badge; an unscanned one shows muted "not scanned". scope marks
// whether the scan is for the exact requested version ("version") or the
// package across all versions ("package", shown with a "pkg" suffix), the
// latter used when the requested version is unknown.
export function ApprovalVulnBadge({ severity, ids, scope }: { severity?: string; ids?: string[]; scope?: string }) {
  if (severity === undefined) return <span className="text-muted-foreground">not scanned</span>;
  const pkgScope = scope === "package";
  const suffix = pkgScope ? " · pkg" : "";
  const scopeTitle = pkgScope ? "package-level scan (requested version unknown)" : "scan for the requested version";
  if (severity === "none")
    return <SeverityBadge severity="none" title={scopeTitle}>clean{suffix}</SeverityBadge>;
  const count = ids?.length ?? 0;
  return (
    <SeverityBadge severity={severity} title={`${count ? ids!.join(", ") : severity} · ${scopeTitle}`}>
      {severity}{count > 1 ? ` ×${count}` : ""}{suffix}
    </SeverityBadge>
  );
}

// Severity order (worst first) and segment colours for the mini bar.
const SEV_ORDER = ["critical", "high", "medium", "low"] as const;
export const SEV_COLOR: Record<string, string> = {
  critical: "var(--fx-severity-critical)",
  high: "var(--fx-severity-high)",
  medium: "var(--fx-severity-medium)",
  low: "var(--fx-severity-low)",
};
const SEV_BG_CLASS: Record<string, string> = {
  critical: "bg-[var(--fx-severity-critical)]",
  high: "bg-[var(--fx-severity-high)]",
  medium: "bg-[var(--fx-severity-medium)]",
  low: "bg-[var(--fx-severity-low)]",
};

// SeverityBar renders the per-level advisory counts as a segmented stacked bar
// (segment width proportional to count, coloured by severity). size "sm" (the
// list) shows a narrow bar with the bare count on the right; size "lg" (the
// detail page) shows a wide bar with an "N vulns" label. "not scanned" and
// "clean" reuse the badge styling; a scanned result without a per-level
// histogram (older scan) falls back to the single badge.
//
// Hovering the bar opens a detailed popover: a wider segmented bar plus a
// per-severity count breakdown (how many critical/high/medium/low). The popover
// is fixed-positioned from the trigger's rect so it is never clipped by a
// scrolling table container.
export function SeverityBar({ severity, counts, scope, source, scannedAt, size = "sm" }: {
  severity?: string; counts?: Record<string, number>; scope?: string;
  source?: string; scannedAt?: string | null; size?: "sm" | "lg";
}) {
  const [pop, setPop] = useState(false);
  // Unscanned has no provenance to show, so it stays a plain muted label.
  if (severity === undefined) return <span className="text-muted-foreground">not scanned</span>;
  const c = counts ?? {};
  const total = SEV_ORDER.reduce((n, s) => n + (c[s] ?? 0), 0);
  // Clean = scanned with no advisories (severity "none"), or a scanned result
  // without a per-level histogram. Both render the green badge but still open a
  // popover with the scan provenance (source + when).
  const clean = severity === "none" || total === 0;
  const suffix = scope === "package" ? " · pkg" : "";
  const label = size === "lg" ? `${total} vuln${total !== 1 ? "s" : ""}${suffix}` : `${total}`;
  const open = () => setPop(true);
  const segs = SEV_ORDER.flatMap((s) =>
    Array.from({ length: c[s] ?? 0 }, (_, i) => (
      <span key={`${s}-${i}`} className={cn("h-full min-w-[3px] flex-1", SEV_BG_CLASS[s])} />
    ))
  );
  return (
    <span className={cn("relative inline-flex cursor-help items-center gap-2 outline-none", size === "lg" && "gap-3")} tabIndex={0}
      onMouseEnter={open} onMouseLeave={() => setPop(false)}
      onFocus={open} onBlur={() => setPop(false)}>
      {clean ? (
        <SeverityBadge severity="none">clean{suffix}</SeverityBadge>
      ) : (
        <>
          <span
            className={cn(
              "inline-flex overflow-hidden rounded bg-border",
              size === "lg" ? "h-4 w-[280px] rounded-lg max-[760px]:w-[180px]" : "h-1.5 w-[54px]"
            )}
          >
            {segs}
          </span>
          <span className={cn("text-xs text-muted-foreground tabular-nums", size === "lg" && "text-sm")}>{label}</span>
        </>
      )}
      {pop && (
        <span
          className="pointer-events-none absolute top-full left-1/2 z-[200] mt-2 flex w-max -translate-x-1/2 flex-col gap-[9px] rounded-[var(--radius)] border border-border bg-[var(--panel-3)] px-3 py-2.5 text-foreground shadow-[var(--fx-overlay-shadow)]"
          role="tooltip"
        >
          <span className="text-xs font-semibold">
            {clean
              ? "No known advisories"
              : `${total} vulnerabilit${total === 1 ? "y" : "ies"}`}
            {scope === "package" ? " · package-level" : ""}
          </span>
          {!clean && <span className="inline-flex h-2.5 w-[200px] overflow-hidden rounded-[5px] bg-border">{segs}</span>}
          {!clean && (
            <span className="flex min-w-[150px] flex-col gap-1">
              {SEV_ORDER.map((s) => (
                <span key={s} className="flex items-center gap-[9px] text-xs leading-[1.2]">
                  <span className={cn("size-[9px] shrink-0 rounded-[2px]", SEV_BG_CLASS[s])} />
                  <span className="capitalize text-foreground">{s}</span>
                  <span className="ml-auto font-semibold tabular-nums">{c[s] ?? 0}</span>
                </span>
              ))}
            </span>
          )}
          <span className="border-t border-border pt-[7px] text-[11px] text-muted-foreground">
            Source {source || "OSV"} · scanned {scannedAt ? new Date(scannedAt).toLocaleString() : "n/a"}
          </span>
        </span>
      )}
    </span>
  );
}

// repoLink renders a repository name as a link to its detail Approvals tab when
// the id is known, falling back to plain text (non-admin approvers can't list
// repositories, and the detail page is admin-only anyway).
function repoLink(name: string, ids: Record<string, number>): ReactNode {
  const id = ids[name];
  return id ? <Link to="/workspace/repositories/$id/$tab" params={{ id: String(id), tab: "approvals" }}>{name}</Link> : name;
}

const PAGE = 50;
const STATUSES = ["pending", "approved", "rejected"];

// Approvals is the cross-repository work queue for package approval requests:
// security engineers review demand here, approve or reject with a note, and
// pre-approve packages before anyone asks. Per-repository views reuse
// ApprovalList from the repository detail's Approvals tab.
export function Approvals() {
  const [repo, setRepo] = useState("");
  const [repos, setRepos] = useState<Repository[]>([]);
  const [rows, setRows] = useState<Approval[]>([]);
  const [reloadKey, setReloadKey] = useState(0);
  const [preApproving, setPreApproving] = useState(false);

  useEffect(() => {
    // Repository listing is admin-only; non-admin approvers fall back to repo
    // names seen in the approval rows themselves.
    api.listRepositories()
      .then((r) => setRepos(r.filter((x) => x.type === "proxy")))
      .catch(() => setRepos([]));
  }, []);

  const repoOptions = useMemo(() => {
    const names = new Set(repos.map((r) => r.name));
    rows.forEach((a) => names.add(a.repo_name));
    return [...names].sort();
  }, [repos, rows]);

  const repoIdByName = useMemo(() => {
    const m: Record<string, number> = {};
    repos.forEach((r) => { m[r.name] = r.id; });
    return m;
  }, [repos]);

  return (
    <>
      <PageHeader
        title="Approvals"
        actions={
        <Button onClick={() => setPreApproving(true)}>Add decision</Button>
        }
      />
      <PageDescription>
        Quarantine queue for proxied packages. Approve or reject pending requests before a
        proxy serves them, and block specific poisoned versions outright.
      </PageDescription>
      <ApprovalList
        repo={repo}
        showRepo
        reloadKey={reloadKey}
        onRows={setRows}
        repoNames={repoOptions}
        repoIds={repoIdByName}
        filters={
          <Select className="w-full sm:w-[200px]" value={repo} onChange={setRepo}
            options={[
              { value: "", label: "all repositories" },
              ...repoOptions.map((name) => ({ value: name, label: name })),
            ]} />
        }
      />
      {preApproving && (
        <PreApproveModal
          repoNames={repoOptions}
          onDone={() => { setPreApproving(false); setReloadKey((k) => k + 1); }}
          onCancel={() => setPreApproving(false)}
        />
      )}
      <VersionDenies repo={repo} showRepo repoNames={repoOptions} repoIds={repoIdByName} />
    </>
  );
}

// ApprovalList renders the approval queue scoped to an optional repository:
// status filter, table, pagination and the approve/reject decision modal.
// Shared by the global Approvals page and the repository detail tab.
export function ApprovalList({ repo = "", showRepo = true, reloadKey = 0, onRows, filters, repoNames = [], repoIds = {} }: {
  repo?: string;
  showRepo?: boolean;
  reloadKey?: number;
  onRows?: (rows: Approval[]) => void;
  filters?: ReactNode;
  // Proxy repository names the bulk-approve modal can target. Empty disables it.
  repoNames?: string[];
  // Maps repository name to id so the Repository column can link to its detail.
  repoIds?: Record<string, number>;
}) {
  const [status, setStatus] = useState("pending");
  const [rows, setRows] = useState<Approval[]>([]);
  const [count, setCount] = useState(0);
  const [pendingCount, setPendingCount] = useState(0);
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState("");
  const [approvingAll, setApprovingAll] = useState(false);

  useEffect(() => { setOffset(0); }, [repo]);

  const load = useCallback(() => {
    api.listApprovals(repo, status, PAGE, offset)
      .then((res) => { setRows(res.approvals); setCount(res.count); onRows?.(res.approvals); })
      .catch((err) => setError((err as Error).message));
    // Pending count for the targetable scope drives the "Approve all" button.
    api.approvalCount("pending", repo).then((c) => setPendingCount(c.count)).catch(() => setPendingCount(0));
    // onRows is a state setter from the parent; excluding it keeps load stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repo, status, offset, reloadKey]);

  useEffect(() => { load(); }, [load]);

  return (
    <>
      <div className="flex min-w-0 items-center gap-2 mb-4 max-sm:flex-wrap items-stretch max-sm:flex-col">
        <Select className="w-full sm:w-[160px]" value={status}
          onChange={(v) => { setStatus(v); setOffset(0); }}
          options={[
            ...STATUSES.map((s) => ({ value: s, label: s })),
            { value: "", label: "all statuses" },
          ]} />
        {filters}
        <span className="flex items-center text-sm text-muted-foreground">{count} total</span>
        {/* Bulk approve is always offered; the repository is chosen inside the
            modal (defaulting to the active filter), so it works from the global
            queue too. Hidden only when there are no proxy repos to target. */}
        {repoNames.length > 0 && (
          <Button className="sm:ml-auto"
            disabled={pendingCount === 0}
            title={pendingCount === 0 ? "No pending approvals" : undefined}
            onClick={() => setApprovingAll(true)}>Approve all pending</Button>
        )}
      </div>
      {error && <Alert className="mb-4">{error}</Alert>}
      {rows.length === 0 ? (
        <p className="m-0 text-sm text-muted-foreground">No {status || "approval"} requests.</p>
      ) : (
        <TableWrap>
          <Table>
            <TableHeader>
              <TableRow>
                {showRepo && <TableHead>Repository</TableHead>}
                <TableHead>Package</TableHead><TableHead>Version</TableHead><TableHead>Vuln</TableHead><TableHead>Requested by</TableHead><TableHead>Requests</TableHead>
                <TableHead>Last requested</TableHead><TableHead>Status</TableHead><TableHead></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((a) => (
                <TableRow key={a.id}>
                  {showRepo && <TableCell>{repoLink(a.repo_name, repoIds)}</TableCell>}
                  <TableCell className="font-mono text-xs">{a.package}</TableCell>
                  <TableCell className="font-mono text-xs">{a.last_requested_version || <span className="text-muted-foreground">unknown</span>}</TableCell>
                  <TableCell><SeverityBar severity={a.vuln_severity} counts={a.vuln_counts} scope={a.vuln_scope} source={a.vuln_source} scannedAt={a.vuln_scanned_at} /></TableCell>
                  <TableCell>{a.requested_by || <span className="text-muted-foreground">anonymous</span>}</TableCell>
                  <TableCell>{a.request_count}</TableCell>
                  <TableCell className="text-muted-foreground">{new Date(a.last_requested_at).toLocaleString()}</TableCell>
                  <TableCell>
                    <ApprovalStatusBadge status={a.status} title={a.note ? `${a.decided_by}: ${a.note}` : a.decided_by} />
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-right">
                    <Button
                      render={<Link to="/workspace/approvals/$id" params={{ id: String(a.id) }} />}
                      nativeButton={false}
                    >
                      Review
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableWrap>
      )}
      {count > PAGE && (
        <div className="mt-3 flex min-w-0 items-center gap-2 max-sm:flex-col max-sm:items-stretch max-sm:flex-wrap">
          <Button variant="outline" disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE))}>Newer</Button>
          <Button variant="outline" disabled={offset + PAGE >= count}
            onClick={() => setOffset(offset + PAGE)}>Older</Button>
          <span className="text-sm text-muted-foreground">{offset + 1}–{Math.min(offset + PAGE, count)} of {count}</span>
        </div>
      )}
      {approvingAll && (
        <ApproveAllModal
          repoNames={repoNames}
          initialRepo={repo}
          onDone={() => { setApprovingAll(false); load(); }}
          onCancel={() => setApprovingAll(false)}
        />
      )}
    </>
  );
}

// ApproveAllModal approves every pending package in one repository at once. The
// repository is chosen here (defaulting to the active filter), and the pending
// count for the selection is fetched live so the operator sees the blast radius.
function ApproveAllModal({ repoNames, initialRepo, onDone, onCancel }: {
  repoNames: string[];
  initialRepo?: string;
  onDone: () => void;
  onCancel: () => void;
}) {
  const [repo, setRepo] = useState(initialRepo || repoNames[0] || "");
  const [note, setNote] = useState("");
  const [pending, setPending] = useState<number | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!repo) { setPending(null); return; }
    let live = true;
    setPending(null);
    api.approvalCount("pending", repo)
      .then((r) => { if (live) setPending(r.count); })
      .catch(() => { if (live) setPending(null); });
    return () => { live = false; };
  }, [repo]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await api.approveAllPending(repo, note);
      onDone();
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  };

  const single = repoNames.length === 1;
  return (
    <div className="fixed inset-0 z-100 flex items-center justify-center bg-black/70 backdrop-blur-[3px]" onClick={onCancel}>
      <div className="w-[380px] max-w-[90vw] rounded-lg border border-border bg-card p-5 shadow-[var(--fx-overlay-shadow)]" onClick={(e) => e.stopPropagation()}>
        <h2 className="m-0 mb-4 text-base font-semibold">Approve all pending</h2>
        <form onSubmit={submit} className="space-y-4">
          <Field>
          <FieldLabel>Proxy repository</FieldLabel>
          {single ? (
            <Input value={repo} disabled />
          ) : (
            <Select value={repo} onChange={setRepo}
              options={repoNames.map((name) => ({ value: name, label: name }))} />
          )}
          </Field>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {pending === null
              ? "Counting pending requests…"
              : pending === 0
                ? `No pending requests on ${repo}.`
                : `All ${pending} pending ${pending === 1 ? "request" : "requests"} on ${repo} will be approved and served (age policy still applies). This cannot be undone in bulk.`}
          </p>
          <Field>
          <FieldLabel>Note (optional)</FieldLabel>
          <Input value={note} autoFocus placeholder="reason for the record"
            onChange={(e) => setNote(e.target.value)} />
          </Field>
          {error && <Alert>{error}</Alert>}
          <div className="flex min-w-0 items-center justify-end gap-2 max-sm:flex-wrap max-sm:flex-col max-sm:items-stretch">
            <Button variant="outline" type="button" onClick={onCancel}>Cancel</Button>
            <Button type="submit" disabled={busy || !repo || !pending}>
              {busy ? "Approving…" : pending ? `Approve ${pending}` : "Approve"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ReviewModal makes the approve/reject decision with an optional note (in-app,
// never a native dialog). It offers both actions so the reviewer decides in one
// place; the button matching the current status is hidden (re-approving an
// approved package is a no-op). Shared by the queue and the detail page.
export function ReviewModal({ row, onDone, onCancel }: {
  row: Approval;
  onDone: () => void;
  onCancel: () => void;
}) {
  const [note, setNote] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<"approve" | "reject" | null>(null);

  const decide = async (action: "approve" | "reject") => {
    setBusy(action);
    setError("");
    try {
      if (action === "approve") await api.approveApproval(row.id, note);
      else await api.rejectApproval(row.id, note);
      onDone();
    } catch (err) {
      setError((err as Error).message);
      setBusy(null);
    }
  };

  return (
    <div className="fixed inset-0 z-100 flex items-center justify-center bg-black/70 backdrop-blur-[3px]" onClick={onCancel}>
      <div className="w-[380px] max-w-[90vw] rounded-lg border border-border bg-card p-5 shadow-[var(--fx-overlay-shadow)]" onClick={(e) => e.stopPropagation()}>
        <h2 className="m-0 mb-3 text-base font-semibold">Review "{row.package}"</h2>
        <p className="text-sm leading-relaxed text-muted-foreground">
          Approve to serve all versions from {row.repo_name} (age policy still
          applies); reject to block the package, including already-cached content.
        </p>
        <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap mb-3 max-sm:flex-col max-sm:items-stretch">
          <span className="text-sm text-muted-foreground">Vulnerabilities:</span>
          <ApprovalVulnBadge severity={row.vuln_severity} ids={row.vuln_ids} scope={row.vuln_scope} />
        </div>
        <Field>
        <FieldLabel>Note (optional)</FieldLabel>
        <Input value={note} autoFocus placeholder="reason for the record"
          onChange={(e) => setNote(e.target.value)} />
        </Field>
        {error && <Alert className="mt-4">{error}</Alert>}
        <div className="flex min-w-0 items-center gap-2 mt-4 max-sm:flex-wrap justify-end max-sm:flex-col max-sm:items-stretch">
          <Button variant="outline" type="button" onClick={onCancel}>Cancel</Button>
          {row.status !== "rejected" && (
            <Button variant="destructive" type="button" disabled={busy !== null}
              onClick={() => decide("reject")}>{busy === "reject" ? "Rejecting…" : "Reject"}</Button>
          )}
          {row.status !== "approved" && (
            <Button type="button" disabled={busy !== null}
              onClick={() => decide("approve")}>{busy === "approve" ? "Approving…" : "Approve"}</Button>
          )}
        </div>
      </div>
    </div>
  );
}

// VersionDenies is the per-version deny list: the package stays approved while
// single poisoned releases are cut off (incident response, IOC blocking). The
// deny overrides package approval and blocks already-cached copies immediately.
// Shared by the global Approvals page and the repository detail tab.
export function VersionDenies({ repo = "", showRepo = true, repoNames, repoIds = {} }: {
  repo?: string;
  showRepo?: boolean;
  repoNames: string[];
  // Maps repository name to id so the Repository column can link to its detail.
  repoIds?: Record<string, number>;
}) {
  const [rows, setRows] = useState<VersionDeny[]>([]);
  const [count, setCount] = useState(0);
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState("");
  const [adding, setAdding] = useState(false);
  const [removing, setRemoving] = useState<VersionDeny | null>(null);

  useEffect(() => { setOffset(0); }, [repo]);

  const load = useCallback(() => {
    api.listVersionDenies(repo, PAGE, offset)
      .then((res) => { setRows(res.denies); setCount(res.count); })
      .catch((err) => setError((err as Error).message));
  }, [repo, offset]);

  useEffect(() => { load(); }, [load]);

  const remove = async (d: VersionDeny) => {
    setError("");
    try {
      await api.deleteVersionDeny(d.id);
      setRemoving(null);
      load();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <>
      <div className="mb-4 mt-8 flex items-center justify-between gap-3 max-sm:flex-col max-sm:items-stretch">
        <h2 className="m-0 text-base font-semibold">Version denies</h2>
        <Button variant="destructive" onClick={() => setAdding(true)}>Deny version</Button>
      </div>
      <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
        Blocks one exact version even when the package is approved. Applies
        immediately, including already-cached copies.
      </p>
      {error && <Alert className="mb-4">{error}</Alert>}
      {rows.length === 0 ? (
        <p className="m-0 text-sm text-muted-foreground">No denied versions.</p>
      ) : (
        <TableWrap>
          <Table>
            <TableHeader>
              <TableRow>
                {showRepo && <TableHead>Repository</TableHead>}
                <TableHead>Package</TableHead><TableHead>Version</TableHead><TableHead>Reason</TableHead>
                <TableHead>Denied by</TableHead><TableHead>Denied at</TableHead><TableHead></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((d) => (
                <TableRow key={d.id}>
                  {showRepo && <TableCell>{repoLink(d.repo_name, repoIds)}</TableCell>}
                  <TableCell className="font-mono text-xs">{d.package}</TableCell>
                  <TableCell className="font-mono text-xs">{d.version}</TableCell>
                  <TableCell>{d.reason || <span className="text-muted-foreground">none</span>}</TableCell>
                  <TableCell>{d.created_by || <span className="text-muted-foreground">unknown</span>}</TableCell>
                  <TableCell className="text-muted-foreground">{new Date(d.created_at).toLocaleString()}</TableCell>
                  <TableCell className="text-right">
                    <Button variant="outline" onClick={() => setRemoving(d)}>Remove</Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableWrap>
      )}
      {count > PAGE && (
        <div className="mt-3 flex min-w-0 items-center gap-2 max-sm:flex-col max-sm:items-stretch max-sm:flex-wrap">
          <Button variant="outline" disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - PAGE))}>Newer</Button>
          <Button variant="outline" disabled={offset + PAGE >= count}
            onClick={() => setOffset(offset + PAGE)}>Older</Button>
          <span className="text-sm text-muted-foreground">{offset + 1}–{Math.min(offset + PAGE, count)} of {count}</span>
        </div>
      )}
      {adding && (
        <DenyVersionModal
          repoNames={repoNames}
          initialRepo={repo}
          onDone={() => { setAdding(false); load(); }}
          onCancel={() => setAdding(false)}
        />
      )}
      <ConfirmModal
        open={removing !== null}
        title="Remove deny entry"
        message={removing
          ? `${removing.package}@${removing.version} on ${removing.repo_name} will be served again (approval and age policies still apply).`
          : undefined}
        confirmLabel="Remove"
        onConfirm={() => removing && remove(removing)}
        onCancel={() => setRemoving(null)}
      />
    </>
  );
}

// DenyVersionModal blocks one exact (package, version) in a proxy repository.
function DenyVersionModal({ repoNames, initialRepo, onDone, onCancel }: {
  repoNames: string[];
  initialRepo?: string;
  onDone: () => void;
  onCancel: () => void;
}) {
  const [repo, setRepo] = useState(initialRepo || (repoNames[0] ?? ""));
  const [pkg, setPkg] = useState("");
  const [version, setVersion] = useState("");
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      await api.createVersionDeny({ repo, package: pkg.trim(), version: version.trim(), reason });
      onDone();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <div className="fixed inset-0 z-100 flex items-center justify-center bg-black/70 backdrop-blur-[3px]" onClick={onCancel}>
      <div className="w-[380px] max-w-[90vw] rounded-lg border border-border bg-card p-5 shadow-[var(--fx-overlay-shadow)]" onClick={(e) => e.stopPropagation()}>
        <h2 className="m-0 mb-3 text-base font-semibold">Deny version</h2>
        <p className="text-sm leading-relaxed text-muted-foreground">
          Only this exact version is blocked; other versions keep flowing.
          Cached copies stop being served immediately.
        </p>
        <form onSubmit={submit} className="space-y-4">
          <Field>
          <FieldLabel>Proxy repository</FieldLabel>
          <Select value={repo} onChange={setRepo}
            options={repoNames.map((name) => ({ value: name, label: name }))} />
          </Field>
          <Field>
          <FieldLabel>Package</FieldLabel>
          <Input value={pkg} placeholder="lodash, @scope/pkg, group:artifact…"
            onChange={(e) => setPkg(e.target.value)} />
          </Field>
          <Field>
          <FieldLabel>Version</FieldLabel>
          <Input value={version} placeholder="4.17.99 (go modules: v1.2.3)"
            onChange={(e) => setVersion(e.target.value)} />
          </Field>
          <Field>
          <FieldLabel>Reason (optional)</FieldLabel>
          <Input value={reason} placeholder="CVE, IOC, incident reference…"
            onChange={(e) => setReason(e.target.value)} />
          </Field>
          {error && <Alert>{error}</Alert>}
          <div className="flex min-w-0 items-center justify-end gap-2 max-sm:flex-wrap max-sm:flex-col max-sm:items-stretch">
            <Button variant="outline" type="button" onClick={onCancel}>Cancel</Button>
            <Button variant="destructive" type="submit" disabled={!repo || !pkg.trim() || !version.trim()}>
              Deny
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}

// PreApproveModal records a decision for a package nobody has requested yet.
function PreApproveModal({ repoNames, onDone, onCancel }: {
  repoNames: string[];
  onDone: () => void;
  onCancel: () => void;
}) {
  const [repo, setRepo] = useState(repoNames[0] ?? "");
  const [pkg, setPkg] = useState("");
  const [decision, setDecision] = useState("approved");
  const [note, setNote] = useState("");
  const [error, setError] = useState("");

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      await api.createApproval({ repo, package: pkg.trim(), status: decision, note });
      onDone();
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <div className="fixed inset-0 z-100 flex items-center justify-center bg-black/70 backdrop-blur-[3px]" onClick={onCancel}>
      <div className="w-[380px] max-w-[90vw] rounded-lg border border-border bg-card p-5 shadow-[var(--fx-overlay-shadow)]" onClick={(e) => e.stopPropagation()}>
        <h2 className="m-0 mb-4 text-base font-semibold">Add decision</h2>
        <form onSubmit={submit} className="space-y-4">
          <Field>
          <FieldLabel>Proxy repository</FieldLabel>
          <Select value={repo} onChange={setRepo}
            options={repoNames.map((name) => ({ value: name, label: name }))} />
          </Field>
          <Field>
          <FieldLabel>Package</FieldLabel>
          <Input value={pkg} placeholder="lodash, @scope/pkg, group:artifact…"
            onChange={(e) => setPkg(e.target.value)} />
          </Field>
          <Field>
          <FieldLabel>Decision</FieldLabel>
          <Select value={decision} onChange={setDecision}
            options={[
              { value: "approved", label: "approved" },
              { value: "rejected", label: "rejected" },
            ]} />
          </Field>
          <Field>
          <FieldLabel>Note (optional)</FieldLabel>
          <Input value={note} onChange={(e) => setNote(e.target.value)} />
          </Field>
          {error && <Alert>{error}</Alert>}
          <div className="flex min-w-0 items-center justify-end gap-2 max-sm:flex-wrap max-sm:flex-col max-sm:items-stretch">
            <Button variant="outline" type="button" onClick={onCancel}>Cancel</Button>
            <Button type="submit" disabled={!repo || !pkg.trim()}>Save</Button>
          </div>
        </form>
      </div>
    </div>
  );
}
