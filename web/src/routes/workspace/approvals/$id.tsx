import { ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import { createFileRoute, Navigate, useNavigate, useParams } from "@tanstack/react-router";
import { Approval, api } from "@/api";
import { useAuth } from "@/authContext";
import { ReviewModal, SeverityBar } from "./index";
import { Alert } from "@/components/app-ui/alert";
import { PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ApprovalStatusBadge } from "@/components/app-ui/status-badge";
import { SeverityBadge } from "@/components/app-ui/severity-badge";
import { Badge } from "@/components/app-ui/badge";
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
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/workspace/approvals/$id")({
  component: ApprovalDetailRoute,
});

function ApprovalDetailRoute() {
  const { me } = useAuth();
  return me.admin || me.approver || me.auditor ? <ApprovalDetail /> : <Navigate to="/workspace/repositories" replace />;
}

// ApprovalDetail is the per-request review screen: it shows the full approval
// metadata and the OSV vulnerability analysis (OVS) so a reviewer can judge the
// package before deciding. The decision itself is made in the shared ReviewModal.
export function ApprovalDetail() {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id?: string };
  const navigate = useNavigate();
  const approvalId = Number(id);
  const [row, setRow] = useState<Approval | null>(null);
  const [error, setError] = useState("");
  const [reviewing, setReviewing] = useState(false);

  const load = useCallback(() => {
    api.getApproval(approvalId)
      .then(setRow)
      .catch((e) => setError((e as Error).message));
  }, [approvalId]);

  useEffect(() => { load(); }, [load]);

  if (error && !row) return <Alert className="my-2.5">{error}</Alert>;
  if (!row) return <div className="text-sm text-muted-foreground">{t("common.loading")}</div>;

  return (
    <>
      <PageHeader
        title={
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <span className="font-mono">{row.package}</span>
            <ApprovalStatusBadge status={row.status} />
          </div>
        }
        actions={
          <>
          <Button onClick={() => setReviewing(true)}>{t("common.review")}</Button>
          <Button variant="outline" onClick={() => navigate({ to: "/workspace/approvals" })}>
            {t("approval.back")}
          </Button>
          </>
        }
      />
      {error && <Alert className="mb-4">{error}</Alert>}

      <Card size="sm" className="mb-4">
        <CardContent>
        <h2 className="m-0 mb-4 text-base font-semibold">{t("common.request")}</h2>
        <dl className="m-0 grid grid-cols-[max-content_1fr] gap-x-5 gap-y-2 [&_dd]:m-0 [&_dt]:text-muted-foreground">
          <dt>{t("common.repository")}</dt><dd>{row.repo_name}</dd>
          <dt>{t("common.package")}</dt><dd className="font-mono">{row.package}</dd>
          <dt>{t("approval.requested-version")}</dt>
          <dd className="font-mono">
            {row.last_requested_version || <span className="text-muted-foreground">{t("approval.version-unknown")}</span>}
          </dd>
          <dt>{t("approval.requested-by")}</dt><dd>{row.requested_by || <span className="text-muted-foreground">{t("common.anonymous")}</span>}</dd>
          <dt>{t("common.requests")}</dt><dd>{row.request_count}</dd>
          <dt>{t("approval.first-requested")}</dt><dd className="text-muted-foreground">{new Date(row.first_requested_at).toLocaleString()}</dd>
          <dt>{t("approval.last-requested")}</dt><dd className="text-muted-foreground">{new Date(row.last_requested_at).toLocaleString()}</dd>
          {row.decided_by && <><dt>{t("approval.decided-by")}</dt><dd>{row.decided_by}</dd></>}
          {row.decided_at && <><dt>{t("approval.decided-at")}</dt><dd className="text-muted-foreground">{new Date(row.decided_at).toLocaleString()}</dd></>}
          {row.note && <><dt>{t("common.note")}</dt><dd>{row.note}</dd></>}
        </dl>
        </CardContent>
      </Card>

      <OvsAnalysis row={row} />

      <ReviewersPanel reviewers={row.reviewers} />

      {reviewing && (
        <ReviewModal
          row={row}
          onDone={() => { setReviewing(false); load(); }}
          onCancel={() => setReviewing(false)}
        />
      )}
    </>
  );
}

// OvsAnalysis renders the OSV scan result: a large severity bar, the scan
// metadata (result, scope, when, how long), and a table of advisories with
// id, severity, CVSS score and a link to osv.dev. Empty until the async scan
// lands.
function OvsAnalysis({ row }: { row: Approval }) {
  const { t } = useTranslation();
  const advisories = row.vuln_advisories ?? [];
  const ids = row.vuln_ids ?? [];
  const pkgScope = row.vuln_scope === "package";
  const clean = row.vuln_severity === "none";
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">{t("approval.vuln-analysis")}</h2>
      {row.vuln_severity === undefined ? (
        <p className="m-0 text-sm leading-relaxed text-muted-foreground">
          {t("approval.not-scanned")}
        </p>
      ) : (
        <>
          <div className="my-4">
            <SeverityBar severity={row.vuln_severity} counts={row.vuln_counts} scope={row.vuln_scope} source={row.vuln_source} scannedAt={row.vuln_scanned_at} size="lg" />
          </div>
          <dl className="m-0 grid grid-cols-[max-content_1fr] gap-x-5 gap-y-2 [&_dd]:m-0 [&_dt]:text-muted-foreground">
            <dt>{t("approval.data-source")}</dt>
            <dd>
              {!row.vuln_source || row.vuln_source === "OSV"
                ? <a className="underline underline-offset-4 hover:no-underline" href="https://osv.dev" target="_blank" rel="noreferrer">{t("approval.osv-source")}</a>
                : row.vuln_source}
            </dd>
            <dt>{t("common.result")}</dt>
            <dd>{clean ? t("approval.clean") : <>{t("approval.vulnerable-highest")} <strong>{row.vuln_severity}</strong></>}</dd>
            <dt>{t("common.scope")}</dt>
            <dd>{pkgScope ? t("approval.package-level") : `Version ${row.last_requested_version}`}</dd>
            <dt>{t("approval.scanned-at")}</dt>
            <dd className="text-muted-foreground">{row.vuln_scanned_at ? new Date(row.vuln_scanned_at).toLocaleString() : t("common.na")}</dd>
            <dt>{t("common.duration")}</dt>
            <dd className="text-muted-foreground">{row.vuln_scan_ms != null ? `${row.vuln_scan_ms} ms` : t("common.na")}</dd>
          </dl>
          {advisories.length > 0 ? (
            <AdvisoryTable advisories={advisories} />
          ) : ids.length > 0 ? (
            <ul className="mt-2.5 mb-0 columns-2 pl-[18px] [column-gap:28px] max-[760px]:columns-1 [&_li]:break-inside-avoid [&_li]:font-mono [&_li]:text-[13px]">
              {ids.map((vid) => (
                <li key={vid}><a className="underline underline-offset-4 hover:no-underline" href={`https://osv.dev/${vid}`} target="_blank" rel="noreferrer">{vid}</a></li>
              ))}
            </ul>
          ) : (
            <p className="mb-0">{t("approval.no-advisories")}</p>
          )}
        </>
      )}
      </CardContent>
    </Card>
  );
}

type Advisory = { id: string; severity: string; score?: string };
type SortKey = "idx" | "id" | "severity" | "cvss";
const SEV_RANK: Record<string, number> = { critical: 4, high: 3, medium: 2, low: 1 };

// SortIcon is a stacked up/down chevron drawn inline as SVG (no icon library).
// When inactive both chevrons are muted to signal the column is sortable; when
// active the sorted direction is accented and the other dimmed.
function SortIcon({ state }: { state: "asc" | "desc" | null }) {
  const up = state === "asc" ? "var(--accent)" : state === "desc" ? "var(--border)" : "var(--muted)";
  const down = state === "desc" ? "var(--accent)" : state === "asc" ? "var(--border)" : "var(--muted)";
  return (
    <svg className="block shrink-0" width="11" height="14" viewBox="0 0 11 14" aria-hidden="true" focusable="false">
      <path d="M2 5 L5.5 1.5 L9 5" fill="none" stroke={up} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M2 9 L5.5 12.5 L9 9" fill="none" stroke={down} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// AdvisoryTable renders the scan's advisories with every column sortable
// ascending/descending. The header cells are sort buttons; the active column
// shows ▲/▼ and the rest show ↕ to signal they are sortable. # sorts by the
// original (as-scanned) order, severity by rank, and CVSS numerically.
function AdvisoryTable({ advisories }: { advisories: Advisory[] }) {
  const { t } = useTranslation();
  const [key, setKey] = useState<SortKey>("idx");
  const [dir, setDir] = useState<"asc" | "desc">("asc");

  const cvss = (a: Advisory) => {
    const n = parseFloat(a.score ?? "");
    return Number.isNaN(n) ? -1 : n;
  };
  const sorted = useMemo(() => {
    const rows = advisories.map((a, i) => ({ a, i }));
    rows.sort((x, y) => {
      let d = 0;
      switch (key) {
        case "idx": d = x.i - y.i; break;
        case "id": d = x.a.id.localeCompare(y.a.id); break;
        case "severity": d = (SEV_RANK[x.a.severity] ?? 0) - (SEV_RANK[y.a.severity] ?? 0); break;
        case "cvss": d = cvss(x.a) - cvss(y.a); break;
      }
      return dir === "asc" ? d : -d;
    });
    return rows;
  }, [advisories, key, dir]);

  const onSort = (k: SortKey) => {
    if (k === key) setDir(dir === "asc" ? "desc" : "asc");
    else { setKey(k); setDir("asc"); }
  };
  const SortBtn = ({ k, children }: { k: SortKey; children: ReactNode }) => (
    <Button type="button" variant="ghost" size="xs" className="h-auto gap-1 p-0 text-xs uppercase text-inherit hover:bg-transparent hover:text-foreground"
      onClick={() => onSort(k)}
      aria-label={`Sort by ${k}`}>
      {children}
      <SortIcon state={key === k ? dir : null} />
    </Button>
  );

  return (
    <TableWrap className="mt-4">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-14"><SortBtn k="idx">#</SortBtn></TableHead>
            <TableHead><SortBtn k="id">{t("common.advisory-id")}</SortBtn></TableHead>
            <TableHead><SortBtn k="severity">{t("common.severity")}</SortBtn></TableHead>
            <TableHead>
              <SortBtn k="cvss">CVSS</SortBtn>
              <Tooltip>
                <TooltipTrigger render={<span tabIndex={0} aria-label="CVSS help" />}>
                  <span className="ml-[5px] text-[0.85em] text-muted-foreground">ⓘ</span>
                </TooltipTrigger>
                <TooltipContent>
                  {t("approval.cvss-help")}
                </TooltipContent>
              </Tooltip>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {sorted.map(({ a, i }) => (
            <TableRow key={a.id}>
              <TableCell className="text-muted-foreground">{i + 1}</TableCell>
              <TableCell className="font-mono text-xs">
                <a className="underline underline-offset-4 hover:no-underline" href={`https://osv.dev/${a.id}`} target="_blank" rel="noreferrer">{a.id}</a>
              </TableCell>
              <TableCell><SeverityBadge severity={a.severity} /></TableCell>
              <TableCell className="tabular-nums">{a.score || <span className="text-muted-foreground">{t("common.na")}</span>}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableWrap>
  );
}

// ReviewersPanel lists the users permitted to approve this repository, so it is
// clear who can act on the request. OIDC-group approvers who have never signed
// in are not enumerable and so are not shown.
function ReviewersPanel({ reviewers }: { reviewers?: string[] }) {
  const { t } = useTranslation();
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">
        {t("common.reviewers")} <span className="text-xs font-normal text-muted-foreground">{t("repo.approvers-subtitle")}</span>
      </h2>
      {!reviewers || reviewers.length === 0 ? (
        <p className="mb-0 text-sm text-muted-foreground">{t("approval.no-approvers")}</p>
      ) : (
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          {reviewers.map((u) => <Badge key={u}>{u}</Badge>)}
        </div>
      )}
      </CardContent>
    </Card>
  );
}
