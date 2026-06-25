import { useEffect, useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Clock, ShieldCheck } from "lucide-react";
import { api, humanSize, Me, repoEndpoint, Repository } from "@/api";
import { useAuth } from "@/authContext";
import { UpstreamStatus } from "@/components/feedback/upstream-status";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
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
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/workspace/repositories/")({
  component: RepositoriesRoute,
});

function RepositoriesRoute() {
  const { me } = useAuth();
  return <Repositories me={me} />;
}

// ArtifactCount shows the number of stored artifacts in a boxed count; empty
// repos (and group repos, which store nothing themselves) render a muted 0.
// When a proxy has packages awaiting approval, an extra yellow dashed box flags
// the pending count so the repository stands out to an approver.
function ArtifactCount({ repo }: { repo: Repository }) {
  const count = repo.artifact_count ?? 0;
  const pending = repo.pending_approval_count ?? 0;
  const tip = `${pending.toLocaleString()} package${pending === 1 ? "" : "s"} awaiting approval`;
  return (
    <span className="inline-flex items-center gap-1 whitespace-nowrap">
      <Badge variant={count === 0 ? "outline" : "default"}>{count.toLocaleString()}</Badge>
      {pending > 0 && (
        <Badge variant="warning" className="border-dashed" title={tip}>{pending.toLocaleString()}</Badge>
      )}
    </span>
  );
}

// RepoSize shows stored bytes, human-readable (B/KB/MB/GB/TB); proxies with a
// cache size cap also show usage against the cap. Empty repos render a muted 0 B.
function RepoSize({ repo }: { repo: Repository }) {
  const size = repo.total_size ?? 0;
  const max = repo.config.cache.max_size_bytes;
  return (
    <span className={size === 0 ? "text-muted-foreground" : undefined}>
      {humanSize(size)}
      {repo.type === "proxy" && max > 0 && <span className="text-muted-foreground"> / {humanSize(max)}</span>}
    </span>
  );
}

// SecurityIcons renders the supply-chain policy state for a proxy repo: a clock
// (age policy) and a shield (package approval), each lit when enabled and
// carrying a concise tooltip. Non-proxy repos have no upstream to gate.
function SecurityIcons({ repo }: { repo: Repository }) {
  if (repo.type !== "proxy") return <span className="text-muted-foreground">—</span>;
  const age = repo.config.age_policy;
  const approval = repo.config.approval ?? { enabled: false, mode: "enforce" };
  const minAge = age.min_age || "0";
  const ageTip = !age.enabled
    ? "Age policy is disabled."
    : age.action === "warn"
      ? `Age policy warns about versions published less than ${minAge} ago.`
      : `Age policy blocks versions published less than ${minAge} ago.`;
  const approvalTip = !approval.enabled
    ? "Package approval is disabled."
    : approval.mode === "audit"
      ? "Package approval is in audit mode. Requests are logged but packages are still served."
      : "Package approval is on. An admin must approve a package before this proxy serves it.";
  return (
    <span className="inline-flex items-center gap-2">
      <Tooltip>
        <TooltipTrigger render={<span tabIndex={0} aria-label="Age policy" />}>
          <span className={cn("inline-flex text-muted-foreground", age.enabled && "text-primary")}>
            <Clock className="size-4" aria-hidden="true" />
          </span>
        </TooltipTrigger>
        <TooltipContent>{ageTip}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger render={<span tabIndex={0} aria-label="Package approval" />}>
          <span className={cn("inline-flex text-muted-foreground", approval.enabled && "text-primary")}>
            <ShieldCheck className="size-4" aria-hidden="true" />
          </span>
        </TooltipTrigger>
        <TooltipContent>{approvalTip}</TooltipContent>
      </Tooltip>
    </span>
  );
}

// repoCells renders the columns after Name, shared by top-level and nested
// (group member) rows so a member shows its own format/type/endpoint/status.
function repoCells(r: Repository, canViewStatus: boolean) {
  return (
    <>
      <TableCell>{r.format}</TableCell>
      <TableCell>{r.type}</TableCell>
      <TableCell className="overflow-hidden text-ellipsis whitespace-nowrap font-mono text-xs" title={repoEndpoint(r.format, r.name).url}>
        {repoEndpoint(r.format, r.name).url}
        {r.type === "proxy" && !r.config.cache.enabled && <span className="text-muted-foreground"> (cache off)</span>}
      </TableCell>
      <TableCell className="whitespace-nowrap"><ArtifactCount repo={r} /></TableCell>
      <TableCell className="whitespace-nowrap"><RepoSize repo={r} /></TableCell>
      {/* Remote-health and supply-chain policy state are shown to admins and
          auditors (read-only), hidden from plain readers (Nexus parity). */}
      <TableCell>{canViewStatus && r.type === "proxy" ? <UpstreamStatus repoId={r.id} compact /> : <span className="text-muted-foreground">—</span>}</TableCell>
      <TableCell>{canViewStatus ? <SecurityIcons repo={r} /> : <span className="text-muted-foreground">—</span>}</TableCell>
    </>
  );
}

export function Repositories({ me }: { me: Me }) {
  const [repos, setRepos] = useState<Repository[]>([]);
  const [error, setError] = useState("");
  // Detail is read-only browsable by any authenticated user, so every name links
  // into it; admin-only controls are hidden inside the detail page itself.
  const nameNode = (id: number, name: string) => <Link to="/workspace/repositories/$id" params={{ id: String(id) }}>{name}</Link>;
  // Upstream health and security policy columns are visible to admins and
  // auditors (read-only); plain readers see a muted dash.
  const canViewStatus = Boolean(me.admin || me.auditor);
  // Groups are expanded by default so the composition tree is visible at a glance.
  const [expanded, setExpanded] = useState<Set<number>>(new Set());

  const load = () =>
    api.listRepositories()
      .then((rs) => {
        setRepos(rs);
        setExpanded(new Set(rs.filter((r) => r.type === "group").map((r) => r.id)));
      })
      .catch((e) => setError(e.message));
  useEffect(() => { load(); }, []);

  const byName = Object.fromEntries(repos.map((r) => [r.name, r]));
  // Names that belong to at least one group are shown only nested under their
  // group(s), never as a duplicate top-level row.
  const memberNames = new Set(
    repos.flatMap((r) => (r.type === "group" ? r.config.group?.members ?? [] : [])),
  );
  const topLevel = repos.filter((r) => !memberNames.has(r.name));
  const toggle = (id: number) =>
    setExpanded((s) => {
      const next = new Set(s);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });

  return (
    <>
      <PageHeader
        title="Repositories"
        actions={me.admin && (
          <Button render={<Link to="/workspace/repositories/new" />} nativeButton={false}>
            New repository
          </Button>
        )}
      />
      <PageDescription>
        Host and proxy artifacts across Maven, npm, Cargo, Go, and PyPI. Configure
        per-repository caching and supply-chain policies (age cooldown, package approval).
      </PageDescription>
      {error && <Alert className="mb-4">{error}</Alert>}
      <Card size="sm" className="mb-4">
        <CardContent>
        <TableWrap>
        <Table className="table-fixed">
          <TableHeader>
            <TableRow><TableHead className="w-[16%]">Name</TableHead><TableHead className="w-[8%]">Format</TableHead><TableHead className="w-[8%]">Type</TableHead><TableHead className="w-[31%]">Endpoint (forklift)</TableHead><TableHead className="w-[9%]">Artifacts</TableHead><TableHead className="w-[8%]">Size</TableHead><TableHead className="w-[11%]">Upstream</TableHead><TableHead className="w-[9%]">Security</TableHead></TableRow>
          </TableHeader>
          <TableBody>
            {topLevel.flatMap((r) => {
              const isGroup = r.type === "group";
              const members = r.config.group?.members ?? [];
              const open = expanded.has(r.id);
              const rows = [
                <TableRow key={`r-${r.id}`}>
                  <TableCell className="overflow-hidden text-ellipsis whitespace-nowrap">
                    {isGroup ? (
                      <span className="flex min-w-0 items-center gap-1">
                        <Button type="button" variant="ghost" size="icon-xs" className="size-5 text-muted-foreground hover:text-foreground" aria-expanded={open}
                          aria-label={open ? "Collapse group" : "Expand group"} onClick={() => toggle(r.id)}>
                          {open ? "▾" : "▸"}
                        </Button>
                        {nameNode(r.id, r.name)}
                        <span className="text-xs text-muted-foreground">({members.length})</span>
                      </span>
                    ) : (
                      nameNode(r.id, r.name)
                    )}
                  </TableCell>
                  {repoCells(r, canViewStatus)}
                </TableRow>,
              ];
              if (isGroup && open) {
                members.forEach((name, i) => {
                  const m = byName[name];
                  const last = i === members.length - 1;
                  rows.push(
                    <TableRow key={`r-${r.id}-m-${name}`} className={cn("bg-muted/20", last && "last")}>
                      <TableCell className="overflow-hidden text-ellipsis whitespace-nowrap pl-7">
                        {m
                          ? nameNode(m.id, name)
                          : <span className="text-muted-foreground">{name}</span>}
                      </TableCell>
                      {m
                        ? repoCells(m, canViewStatus)
                        : <TableCell colSpan={7} className="text-muted-foreground">member not found</TableCell>}
                    </TableRow>,
                  );
                });
              }
              return rows;
            })}
            {repos.length === 0 && (
              <TableRow><TableCell colSpan={8} className="text-muted-foreground">No repositories yet.</TableCell></TableRow>
            )}
          </TableBody>
        </Table>
        </TableWrap>
        </CardContent>
      </Card>
    </>
  );
}
