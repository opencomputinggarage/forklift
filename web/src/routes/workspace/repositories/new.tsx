import { FormEvent, useEffect, useState } from "react";
import { createFileRoute, Link, Navigate, useNavigate } from "@tanstack/react-router";
import { api, Repository, UpstreamHealth } from "@/api";
import { useAuth } from "@/authContext";
import { Select } from "@/components/app-ui/select";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/components/app-ui/table";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/workspace/repositories/new")({
  component: RepositoryNewRoute,
});

function RepositoryNewRoute() {
  const { me } = useAuth();
  return me.admin ? <RepositoryNew /> : <Navigate to="/workspace/repositories" replace />;
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
      navigate({ to: "/workspace/repositories" });
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <>
      <PageHeader title="New repository" />
      <PageDescription>
        Register a hosted, proxy, or group repository for package delivery.
      </PageDescription>

      <Card size="sm" className="mb-4 max-w-[44rem]">
        <CardContent>
          <form onSubmit={submit} className="space-y-5">
            <FieldGroup className="gap-4">
              <Field>
                <FieldLabel htmlFor="repository-name">
                  Name<span className="text-destructive">*</span>
                </FieldLabel>
                <Input
                  id="repository-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="maven-central"
                  required
                  autoFocus
                  pattern="[A-Za-z0-9_-]{1,64}"
                  title="Letters, digits, '-' and '_' only (max 64 characters)"
                />
                <FieldDescription>Letters, digits, dash and underscore only.</FieldDescription>
              </Field>

              <Field>
                <FieldLabel>Format<span className="text-destructive">*</span></FieldLabel>
                <Select
                  value={format}
                  onChange={(v) => { setFormat(v); setMembers([]); }}
                  options={[
                    { value: "maven", label: "Maven / Gradle" },
                    { value: "npm", label: "npm" },
                    { value: "cargo", label: "Cargo" },
                    { value: "go", label: "Go Modules" },
                    { value: "pypi", label: "PyPI" },
                  ]}
                />
              </Field>

              <Field>
                <FieldLabel>Type<span className="text-destructive">*</span></FieldLabel>
                <div className="grid gap-2 sm:grid-cols-3" role="radiogroup" aria-label="Repository type">
                  {REPO_TYPES.map((t) => (
                    <Button
                      key={t.value}
                      type="button"
                      variant="ghost"
                      role="radio"
                      aria-checked={type === t.value}
                      className={cn(
                        "h-auto w-full flex-col items-start justify-start whitespace-normal rounded-lg border border-border bg-input px-3.5 py-3 text-left text-sm transition-colors hover:bg-muted",
                        type === t.value && "border-primary bg-primary/10"
                      )}
                      onClick={() => setType(t.value)}
                    >
                      <div className={cn("mb-1 font-semibold", type === t.value && "text-primary")}>{t.title}</div>
                      <div className="text-xs leading-relaxed text-muted-foreground">{t.desc}</div>
                    </Button>
                  ))}
                </div>
              </Field>
            </FieldGroup>

            {type === "proxy" && (
              <div className="space-y-3 border-t border-border pt-4">
                <div>
                  <h2 className="m-0 text-sm font-semibold">Proxy upstream</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Configure the remote registry and optional supply-chain cooldown.
                  </p>
                </div>

                <FieldGroup className="gap-4">
                  <Field>
                    <FieldLabel htmlFor="upstream-url">
                      Upstream URL<span className="text-destructive">*</span>
                    </FieldLabel>
                    <Input
                      id="upstream-url"
                      value={upstream}
                      onChange={(e) => setUpstream(e.target.value)}
                      placeholder="https://repo1.maven.org/maven2"
                      required
                    />
                    <ConnectivityHint checking={checking} health={health} hasUrl={upstream.trim() !== ""} />
                  </Field>

                  <Field>
                    <FieldLabel>Age policy</FieldLabel>
                    <label data-slot="checkbox-label" className="inline-flex items-center gap-2 rounded-lg border border-border bg-background/40 px-3 py-2 text-sm">
                      <Checkbox checked={ageEnabled} onCheckedChange={(checked) => setAgeEnabled(checked === true)} />
                      <span>Block versions newer than a cooldown window</span>
                    </label>
                  </Field>

                  {ageEnabled && (
                    <Field>
                      <FieldLabel htmlFor="minimum-age">
                        Minimum age<span className="text-destructive">*</span>
                      </FieldLabel>
                      <Input
                        id="minimum-age"
                        value={minAge}
                        onChange={(e) => setMinAge(e.target.value)}
                        placeholder="3d"
                        required
                      />
                      <FieldDescription>Examples: 3d, 72h, 2w.</FieldDescription>
                    </Field>
                  )}
                </FieldGroup>
              </div>
            )}

            {type === "group" && (
              <div className="space-y-3 border-t border-border pt-4">
                <div>
                  <h2 className="m-0 text-sm font-semibold">Members</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Select member repositories in lookup order. The first hit wins.
                  </p>
                </div>

                <div className="rounded-lg border border-border/80 bg-background/40 p-3">
                  <MemberList
                    members={members}
                    onChange={setMembers}
                    repoIndex={Object.fromEntries(repos.map((r) => [r.name, r.id]))}
                    repoTypes={Object.fromEntries(repos.map((r) => [r.name, r.type]))}
                  />
                  <div className="flex min-w-0 items-center gap-2 mt-3 max-sm:flex-wrap">
                    <Select
                      value=""
                      placeholder="add member..."
                      onChange={(v) => v && setMembers([...members, v])}
                      options={candidates.map((r) => ({ value: r.name, label: `${r.name} (${r.type})` }))}
                    />
                  </div>
                  {candidates.length === 0 && members.length === 0 && (
                    <p className="mt-3 text-sm text-muted-foreground">
                      No {format} repositories exist yet. Create the members first.
                    </p>
                  )}
                </div>
              </div>
            )}

            {error && <Alert>{error}</Alert>}
            <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap border-t border-border pt-4">
              <Button type="submit" disabled={!valid}>Create repository</Button>
              <Button variant="outline" type="button" onClick={() => navigate({ to: "/workspace/repositories" })}>Cancel</Button>
            </div>
          </form>
        </CardContent>
      </Card>
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
                ? <Link to="/workspace/repositories/$id" params={{ id: String(id) }}>{name}</Link>
                : name}
            </TableCell>
            <TableCell>{type ? <Badge>{type}</Badge> : <span className="text-muted-foreground">—</span>}</TableCell>
            <TableCell className="whitespace-nowrap text-right">
              <Button variant="outline" size="sm" type="button" disabled={i === 0}
                title="Move up" onClick={() => move(i, -1)}>↑</Button>{" "}
              <Button variant="outline" size="sm" type="button" disabled={i === members.length - 1}
                title="Move down" onClick={() => move(i, 1)}>↓</Button>{" "}
              <Button variant="destructive" size="sm" type="button" title="Remove member"
                onClick={() => onChange(members.filter((m) => m !== name))}>×</Button>
            </TableCell>
          </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
