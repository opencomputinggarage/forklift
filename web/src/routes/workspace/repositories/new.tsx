import { FormEvent, useEffect, useState } from "react";
import { createFileRoute, Link, Navigate, useNavigate } from "@tanstack/react-router";
import { ArrowDown, ArrowUp, X } from "lucide-react";
import { api, Repository, UpstreamHealth } from "@/api";
import { useAuth } from "@/authContext";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/components/ui/table";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/workspace/repositories/new")({
  component: RepositoryNewRoute,
});

function RepositoryNewRoute() {
  const { me } = useAuth();
  return me.admin ? <RepositoryNew /> : <Navigate to="/workspace/repositories" replace />;
}

const REPO_TYPES = [
  { value: "hosted", titleKey: "repo.type.hosted", descKey: "repo.type.hosted-desc" },
  { value: "proxy", titleKey: "repo.type.proxy", descKey: "repo.type.proxy-desc" },
  { value: "group", titleKey: "repo.type.group", descKey: "repo.type.group-desc" },
] as const;

type SelectOption = { value: string; label: string; description?: string };

function SelectControl({
  value,
  options,
  onChange,
  placeholder,
}: {
  value: string;
  options: SelectOption[];
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  const { t } = useTranslation();
  const selectValue = value === "" && !options.some((o) => o.value === "") ? null : value;
  return (
    <Select items={options} value={selectValue} onValueChange={(next) => onChange(next ?? "")}>
      <SelectTrigger className="w-full">
        <SelectValue placeholder={placeholder ?? ""} />
      </SelectTrigger>
      <SelectContent align="start">
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            <span className="flex min-w-0 flex-col">
              <span>{option.label}</span>
              {option.description && (
                <span className="text-xs leading-4 text-muted-foreground">{option.description}</span>
              )}
            </span>
          </SelectItem>
        ))}
        {options.length === 0 && (
          <div className="px-2 py-1.5 text-sm text-muted-foreground">{t("common.no-options")}</div>
        )}
      </SelectContent>
    </Select>
  );
}

export function RepositoryNew() {
  const { t } = useTranslation();
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
    const ctl = new AbortController();
    setChecking(true);
    const t = setTimeout(() => {
      api.checkUpstream(url, ctl.signal)
        .then((h) => { if (!ctl.signal.aborted) { setHealth(h); setChecking(false); } })
        .catch(() => { if (!ctl.signal.aborted) { setHealth(null); setChecking(false); } });
    }, 600);
    return () => { ctl.abort(); clearTimeout(t); };
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
      <header className="mb-4">
        <h1 className="m-0 text-2xl font-semibold tracking-normal">{t("repo.new")}</h1>
        <p className="mt-1 max-w-[58rem] text-sm leading-relaxed text-muted-foreground">
          {t("repo.new-description")}
        </p>
      </header>

      <Card size="sm" className="mb-4 max-w-[52rem]">
        <CardContent>
          <form onSubmit={submit} className="space-y-5">
            <section>
              <div className="mb-4">
                <h2 className="m-0 text-base font-semibold">{t("repo.basics")}</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {t("repo.basics-description")}
                </p>
              </div>

              <FieldGroup className="gap-4">
              <Field>
                <FieldLabel htmlFor="repository-name">
                  {t("common.name")}<span className="text-destructive">*</span>
                </FieldLabel>
                <Input
                  id="repository-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="maven-central"
                  required
                  autoFocus
                  pattern="[A-Za-z0-9_-]{1,64}"
                  title={t("common.name-rule-64")}
                />
                <FieldDescription>{t("common.name-rule")}</FieldDescription>
              </Field>

              <Field>
                <FieldLabel>{t("common.format")}<span className="text-destructive">*</span></FieldLabel>
                <SelectControl
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
                <FieldLabel>{t("common.type")}<span className="text-destructive">*</span></FieldLabel>
                <div className="grid gap-2 md:grid-cols-3" role="radiogroup" aria-label="Repository type">
                  {REPO_TYPES.map((rt) => (
                    <Button
                      key={rt.value}
                      type="button"
                      variant="ghost"
                      role="radio"
                      aria-checked={type === rt.value}
                      className={cn(
                        "h-full w-full flex-col items-start justify-start whitespace-normal rounded-lg border border-border bg-input px-3.5 py-3 text-left text-sm transition-colors hover:bg-muted",
                        type === rt.value && "border-primary bg-primary/10"
                      )}
                      onClick={() => setType(rt.value)}
                    >
                      <div className={cn("mb-1 font-semibold", type === rt.value && "text-primary")}>{t(rt.titleKey)}</div>
                      <div className="text-xs leading-relaxed text-muted-foreground">{t(rt.descKey)}</div>
                    </Button>
                  ))}
                </div>
              </Field>
              </FieldGroup>
            </section>

            {type === "proxy" && (
              <section className="border-t border-border pt-5">
                <div className="mb-4">
                  <h2 className="m-0 text-base font-semibold">{t("repo.proxy-upstream")}</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {t("repo.proxy-description")}
                  </p>
                </div>

                <FieldGroup className="gap-4">
                  <Field>
                    <FieldLabel htmlFor="upstream-url">
                      {t("repo.upstream-url")}<span className="text-destructive">*</span>
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
                    <FieldLabel>{t("repo.age-policy")}</FieldLabel>
                    <label className="inline-flex items-center gap-2 text-sm">
                      <Checkbox
                        checked={ageEnabled}
                        onCheckedChange={(checked) => setAgeEnabled(checked === true)}
                        aria-label={t("repo.cooldown-label")}
                      />
                      <span>{t("repo.cooldown-label")}</span>
                    </label>
                    <FieldDescription>
                      {t("repo.cooldown-note")}
                    </FieldDescription>
                  </Field>

                  {ageEnabled && (
                    <Field>
                      <FieldLabel htmlFor="minimum-age">
                        {t("repo.minimum-age")}<span className="text-destructive">*</span>
                      </FieldLabel>
                      <Input
                        id="minimum-age"
                        value={minAge}
                        onChange={(e) => setMinAge(e.target.value)}
                        placeholder="3d"
                        required
                      />
                      <FieldDescription>{t("repo.cooldown-examples")}</FieldDescription>
                    </Field>
                  )}
                </FieldGroup>
              </section>
            )}

            {type === "group" && (
              <section className="border-t border-border pt-5">
                <div className="mb-4">
                  <h2 className="m-0 text-base font-semibold">{t("repo.members")}</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {t("repo.members-note")}
                  </p>
                </div>

                <MemberList
                  members={members}
                  onChange={setMembers}
                  repoIndex={Object.fromEntries(repos.map((r) => [r.name, r.id]))}
                  repoTypes={Object.fromEntries(repos.map((r) => [r.name, r.type]))}
                />
                <div className="mt-3 flex min-w-0 items-center gap-2 max-sm:flex-wrap">
                  <SelectControl
                    value=""
                    placeholder={t("repo.add-member-placeholder")}
                    onChange={(v) => v && setMembers([...members, v])}
                    options={candidates.map((r) => ({ value: r.name, label: `${r.name} (${r.type})` }))}
                  />
                </div>
                {candidates.length === 0 && members.length === 0 && (
                  <p className="mt-3 text-sm text-muted-foreground">
                    No {format} repositories exist yet. Create the members first.
                  </p>
                )}
              </section>
            )}

            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            <div className="flex min-w-0 items-center gap-2 border-t border-border pt-5 max-sm:flex-col max-sm:items-stretch">
              <Button type="submit" disabled={!valid}>{t("repo.create")}</Button>
              <Button variant="outline" type="button" onClick={() => navigate({ to: "/workspace/repositories" })}>{t("common.cancel")}</Button>
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
  const { t } = useTranslation();
  if (!hasUrl) return null;
  if (checking) {
    return <p className="mt-1.5 text-sm text-muted-foreground">{t("common.checking-connectivity")}</p>;
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
  const { t } = useTranslation();
  const move = (i: number, dir: -1 | 1) => {
    const j = i + dir;
    if (j < 0 || j >= members.length) return;
    const next = [...members];
    [next[i], next[j]] = [next[j], next[i]];
    onChange(next);
  };
  if (members.length === 0) return <p className="text-muted-foreground">{t("repo.no-members")}</p>;
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
            <TableCell>{type ? <Badge variant="outline">{type}</Badge> : <span className="text-muted-foreground">—</span>}</TableCell>
            <TableCell className="whitespace-nowrap text-right">
              <div className="flex justify-end gap-1">
                <Button variant="outline" size="icon-sm" type="button" disabled={i === 0}
                  title={t("common.move-up")} onClick={() => move(i, -1)}>
                  <ArrowUp className="size-3.5" aria-hidden="true" />
                  <span className="sr-only">{t("common.move-up")}</span>
                </Button>
                <Button variant="outline" size="icon-sm" type="button" disabled={i === members.length - 1}
                  title={t("common.move-down")} onClick={() => move(i, 1)}>
                  <ArrowDown className="size-3.5" aria-hidden="true" />
                  <span className="sr-only">{t("common.move-down")}</span>
                </Button>
                <Button variant="destructive" size="icon-sm" type="button" title={t("repo.remove-member")}
                  onClick={() => onChange(members.filter((m) => m !== name))}>
                  <X className="size-3.5" aria-hidden="true" />
                  <span className="sr-only">{t("repo.remove-member")}</span>
                </Button>
              </div>
            </TableCell>
          </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}
