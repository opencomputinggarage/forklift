import { FormEvent, useEffect, useState } from "react";
import { createFileRoute, Link, Navigate, useNavigate } from "@tanstack/react-router";
import { api, Repository, UpstreamHealth } from "../../api";
import { useAuth } from "../../authContext";
import { Select } from "@/components/app-ui/select";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Inline, Panel, PanelBody } from "@/components/app-ui/page";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/components/app-ui/table";
import { Button, buttonVariants } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/repositories/new")({
  component: RepositoryNewRoute,
});

function RepositoryNewRoute() {
  const { me } = useAuth();
  return me.admin ? <RepositoryNew /> : <Navigate to="/repositories" replace />;
}

const REPO_TYPES = [
  { value: "hosted", title: "Hosted", desc: "Store artifacts uploaded directly by your team" },
  { value: "proxy", title: "Proxy", desc: "Cache and serve artifacts from an upstream registry" },
  { value: "group", title: "Group", desc: "Combine repositories behind a single read-only URL" },
];

export function RepositoryNew() {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [format, setFormat] = useState("maven");
  const [type, setType] = useState("proxy");
  const [upstream, setUpstream] = useState("");
  const [ageEnabled, setAgeEnabled] = useState(false);
  const [minAge, setMinAge] = useState("3d");
  const [members, setMembers] = useState<string[]>([]);
  const [repos, setRepos] = useState<Repository[]>([]);
  const [error, setError] = useState("");
  // Auto connectivity check for the upstream URL (proxy only), debounced.
  const [health, setHealth] = useState<UpstreamHealth | null>(null);
  const [checking, setChecking] = useState(false);

  useEffect(() => {
    api.listRepositories().then(setRepos).catch(() => setRepos([]));
  }, []);

  // Probe the upstream URL ~600ms after the user stops typing. The cancelled
  // flag drops stale responses so only the latest URL's result is shown.
  useEffect(() => {
    const url = upstream.trim();
    if (type !== "proxy" || url === "") {
      setHealth(null);
      setChecking(false);
      return;
    }
    let cancelled = false;
    setChecking(true);
    const t = setTimeout(() => {
      api.checkUpstream(url)
        .then((h) => { if (!cancelled) { setHealth(h); setChecking(false); } })
        .catch(() => { if (!cancelled) { setHealth(null); setChecking(false); } });
    }, 600);
    return () => { cancelled = true; clearTimeout(t); };
  }, [upstream, type]);

  // Candidate members: same format, not a group itself, not yet selected.
  const candidates = repos.filter(
    (r) => r.format === format && r.type !== "group" && !members.includes(r.name),
  );

  // Mirrors the form's required fields so Create stays disabled until complete.
  const valid =
    name.trim() !== "" &&
    (type !== "proxy" || upstream.trim() !== "") &&
    (type !== "proxy" || !ageEnabled || minAge.trim() !== "") &&
    (type !== "group" || members.length > 0);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      await api.createRepository({
        name,
        format,
        type,
        upstream_url: type === "proxy" ? upstream : "",
        config: {
          cache: { enabled: true, metadata_ttl: "15m", negative_ttl: "5m", eviction: "lru" },
          age_policy: ageEnabled
            ? { enabled: true, min_age: minAge, action: "block" }
            : { enabled: false },
          ...(type === "group" ? { group: { members } } : {}),
        },
      });
      navigate({ to: "/repositories" });
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <>
      <h1>New repository</h1>
      <Panel className="max-w-[35rem]">
      <PanelBody>
      <form onSubmit={submit}>
        <label>Name<span className="req">*</span></label>
        <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="maven-central" required
          pattern="[A-Za-z0-9_-]{1,64}" title="Letters, digits, '-' and '_' only (max 64 characters)" />
        <label>Format<span className="req">*</span></label>
        <Select value={format} onChange={(v) => { setFormat(v); setMembers([]); }}
          options={[
            { value: "maven", label: "Maven / Gradle" },
            { value: "npm", label: "npm" },
            { value: "cargo", label: "Cargo" },
            { value: "go", label: "Go Modules" },
            { value: "pypi", label: "PyPI" },
          ]} />
        <label>Type<span className="req">*</span></label>
        <div className="flex gap-2.5" role="radiogroup" aria-label="Repository type">
          {REPO_TYPES.map((t) => (
            <button
              key={t.value}
              type="button"
              role="radio"
              aria-checked={type === t.value}
              className={cn(
                "flex-1 rounded-lg border bg-input px-3.5 py-3 text-left text-sm transition-colors hover:border-border hover:bg-muted",
                type === t.value && "border-primary bg-primary/10"
              )}
              onClick={() => setType(t.value)}
            >
              <div className={cn("mb-1 font-semibold", type === t.value && "text-primary")}>{t.title}</div>
              <div className="text-xs leading-relaxed text-muted-foreground">{t.desc}</div>
            </button>
          ))}
        </div>
        {type === "proxy" && (
          <>
            <label>Upstream URL<span className="req">*</span></label>
            <Input value={upstream} onChange={(e) => setUpstream(e.target.value)}
              placeholder="https://repo1.maven.org/maven2" required />
            <ConnectivityHint checking={checking} health={health} hasUrl={upstream.trim() !== ""} />
          </>
        )}
        {type === "group" && (
          <>
            <label>Members (lookup order, first hit wins)<span className="req">*</span></label>
            <MemberList members={members} onChange={setMembers}
              repoIndex={Object.fromEntries(repos.map((r) => [r.name, r.id]))}
              repoTypes={Object.fromEntries(repos.map((r) => [r.name, r.type]))} />
            <Inline className="mt-2">
              <Select value="" placeholder="add member…"
                onChange={(v) => v && setMembers([...members, v])}
                options={candidates.map((r) => ({ value: r.name, label: `${r.name} (${r.type})` }))} />
            </Inline>
            {candidates.length === 0 && members.length === 0 && (
              <p className="text-muted-foreground">No {format} repositories exist yet. Create the members first.</p>
            )}
          </>
        )}
        {type === "proxy" && (
          <>
            <h2>Age policy (supply-chain cooldown)</h2>
            <div className="flex items-center gap-2">
              <Checkbox checked={ageEnabled} onCheckedChange={(checked) => setAgeEnabled(checked === true)} />
              <span>Block versions newer than a cooldown window</span>
            </div>
            {ageEnabled && (
              <>
                <label>Minimum age (e.g. 3d, 72h)<span className="req">*</span></label>
                <Input value={minAge} onChange={(e) => setMinAge(e.target.value)} required />
              </>
            )}
          </>
        )}
        {error && <Alert className="mt-4">{error}</Alert>}
        <Inline className="mt-5">
          <Button type="submit" disabled={!valid}>Create</Button>
          <Button variant="outline" type="button" onClick={() => navigate({ to: "/repositories" })}>Cancel</Button>
        </Inline>
      </form>
      </PanelBody>
      </Panel>
    </>
  );
}

// ConnectivityHint renders the live result of the debounced upstream probe
// under the URL field: a spinner-ish "checking" line, then reachable/unreachable.
function ConnectivityHint({ checking, health, hasUrl }: {
  checking: boolean; health: UpstreamHealth | null; hasUrl: boolean;
}) {
  if (!hasUrl) return null;
  if (checking) {
    return <p className="mt-1.5 text-sm text-muted-foreground">Checking connectivity…</p>;
  }
  if (!health) return null;
  if (health.reachable) {
    return (
      <p className="mt-1.5 text-sm text-emerald-300">
        ✓ Reachable — HTTP {health.status}{health.latency_ms != null && ` (${health.latency_ms} ms)`}
      </p>
    );
  }
  return (
    <p className="mt-1.5 text-sm text-destructive">
      ✗ Unreachable{health.error ? ` — ${health.error}` : ""}
    </p>
  );
}

// MemberList renders an ordered member list with reorder and remove controls.
// Shared by the create form and the settings tab. When repoIndex maps a member
// name to a repository id, the name links to that repository's page.
export function MemberList({ members, onChange, repoIndex, repoTypes }: {
  members: string[];
  onChange: (m: string[]) => void;
  repoIndex?: Record<string, number>;
  repoTypes?: Record<string, string>;
}) {
  const move = (i: number, dir: -1 | 1) => {
    const j = i + dir;
    if (j < 0 || j >= members.length) return;
    const next = [...members];
    [next[i], next[j]] = [next[j], next[i]];
    onChange(next);
  };
  if (members.length === 0) return <p className="text-muted-foreground">No members selected.</p>;
  return (
    <Table>
      <TableBody>
        {members.map((name, i) => {
          const id = repoIndex?.[name];
          const type = repoTypes?.[name];
          return (
          <TableRow key={name}>
            <TableCell className="w-6 text-muted-foreground">{i + 1}</TableCell>
            <TableCell className="font-mono text-xs">
              {id !== undefined
                ? <Link to="/repositories/$id" params={{ id: String(id) }}>{name}</Link>
                : name}
            </TableCell>
            <TableCell>{type ? <Badge>{type}</Badge> : <span className="text-muted-foreground">—</span>}</TableCell>
            <TableCell className="whitespace-nowrap text-right">
              <button className={buttonVariants({ variant: "outline", size: "sm" })} type="button" disabled={i === 0}
                title="Move up" onClick={() => move(i, -1)}>↑</button>{" "}
              <button className={buttonVariants({ variant: "outline", size: "sm" })} type="button" disabled={i === members.length - 1}
                title="Move down" onClick={() => move(i, 1)}>↓</button>{" "}
              <button className={buttonVariants({ variant: "destructive", size: "sm" })} type="button" title="Remove member"
                onClick={() => onChange(members.filter((m) => m !== name))}>×</button>
            </TableCell>
          </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
