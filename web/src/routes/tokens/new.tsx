import { FormEvent, useEffect, useState } from "react";
import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";
import { api } from "../../api";
import { Combobox } from "../../components/combobox";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Inline, Panel, PanelBody } from "@/components/app-ui/page";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/tokens/new")({
  component: TokenNew,
});

const ACTIONS = ["read", "write", "delete"];
const MAX_TTL_HOURS = 365 * 24;

interface Scope {
  repo_pattern: string;
  actions: string[];
}

function dateStr(d: Date): string {
  return d.toISOString().slice(0, 10);
}

// Token creation page. Reached from the New token button on /tokens
// (self-service for the current user) or from a user's detail page at
// /users/:id/tokens/new (admin creating a token for that user). The presence of
// the :id route param selects the target and where Done/Cancel return to. All
// fields are required; expiry is capped at one year by the API.
export function TokenNew() {
  const navigate = useNavigate();
  const { id } = useParams({ strict: false });
  const forUserId = id ? Number(id) : null;
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [scopes, setScopes] = useState<Scope[]>([]);
  const [expiresOn, setExpiresOn] = useState("");
  const [error, setError] = useState("");
  const [created, setCreated] = useState("");
  const [copied, setCopied] = useState(false);

  // Scope add-row state.
  const [pattern, setPattern] = useState("");
  const [actions, setActions] = useState<string[]>(["read"]);

  // Repository names for scope-pattern autocomplete. Available to any
  // authenticated user; "*" (all repositories) is offered as the first option.
  const [repoOptions, setRepoOptions] = useState<string[]>(["*"]);
  const [repoTypes, setRepoTypes] = useState<Record<string, string>>({});
  useEffect(() => {
    api.listRepositoryNames()
      .then((repos) => {
        setRepoOptions(["*", ...repos.map((r) => r.name)]);
        setRepoTypes(Object.fromEntries(repos.map((r) => [r.name, `${r.format} · ${r.type}`])));
      })
      .catch(() => setRepoOptions(["*"]));
  }, []);

  const today = new Date();
  const minDate = new Date(today.getTime() + 24 * 3600 * 1000);
  const maxDate = new Date(today.getTime() + MAX_TTL_HOURS * 3600 * 1000);

  const toggle = (a: string) =>
    setActions((cur) => cur.includes(a) ? cur.filter((x) => x !== a) : [...cur, a]);

  const addScope = () => {
    if (!pattern.trim() || actions.length === 0) return;
    setScopes((cur) => [...cur, { repo_pattern: pattern.trim(), actions: [...actions] }]);
    setPattern("");
    setActions(["read"]);
  };

  const expiresIn = (): string => {
    const target = new Date(expiresOn + "T00:00:00");
    const hours = Math.ceil((target.getTime() - Date.now()) / 3600000);
    return `${Math.min(Math.max(hours, 1), MAX_TTL_HOURS)}h`;
  };

  const valid = name.trim() && description.trim() && scopes.length > 0 && expiresOn;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      const body = {
        name: name.trim(),
        description: description.trim(),
        scopes,
        expires_in: expiresIn(),
      };
      const res = forUserId !== null
        ? await api.createUserToken(forUserId, body)
        : await api.createToken(body);
      setCreated(res.token);
    } catch (err) {
      setError((err as Error).message);
    }
  };

  const copy = () => {
    navigator.clipboard?.writeText(created);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (created) {
    return (
      <>
        <h1>Token created</h1>
        <Panel className="max-w-[40rem]">
          <PanelBody>
            <p className="mb-3 text-sm text-muted-foreground">Copy this token now; it will not be shown again.</p>
            <Inline className="items-stretch max-sm:flex-col">
              <div className="min-h-8 flex-1 overflow-x-auto rounded-lg border border-border bg-muted px-3 py-2 font-mono text-xs">
                {created}
              </div>
              <Button variant="outline" type="button" onClick={copy}>
                {copied ? "Copied" : "Copy"}
              </Button>
            </Inline>
            <Button className="mt-5" onClick={() => navigate(forUserId ? { to: "/users/$id", params: { id: String(forUserId) } } : { to: "/tokens" })}>Done</Button>
          </PanelBody>
        </Panel>
      </>
    );
  }

  return (
    <>
      <h1>{forUserId !== null ? "Create token for user" : "Create token"}</h1>
      <Panel className="max-w-[40rem]">
        <PanelBody>
          <form onSubmit={submit} className="space-y-4">
            <label className="block text-sm font-medium">Token name<span className="text-destructive">*</span></label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="ci" autoFocus required
              pattern="[A-Za-z0-9_-]{1,64}" title="Letters, digits, '-' and '_' only (max 64 characters)" />

            <label className="block text-sm font-medium">Token description<span className="text-destructive">*</span></label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="What this token is used for" required />

            <div>
              <label className="block text-sm font-medium">Permissions<span className="text-destructive">*</span></label>
              <Inline className="mt-2 flex-wrap gap-1.5">
                {scopes.map((s, i) => (
                  <Badge key={i} className="font-mono">
                    {s.repo_pattern}: {s.actions.join(",")}
                    <button className="ml-1.5 cursor-pointer" type="button" title="Remove permission"
                      onClick={() => setScopes((cur) => cur.filter((_, j) => j !== i))}>x</button>
                  </Badge>
                ))}
                {scopes.length === 0 && <span className="text-sm text-muted-foreground">none yet - add at least one</span>}
              </Inline>
              <Inline className="mt-2 flex-wrap gap-2">
                <Combobox style={{ width: 220 }} value={pattern} onChange={setPattern}
                  options={repoOptions} hints={repoTypes} placeholder="repo pattern (* or maven-*)" />
                {ACTIONS.map((a) => (
                  <label key={a} className="inline-flex items-center gap-1.5 text-xs">
                    <Checkbox checked={actions.includes(a)} onCheckedChange={() => toggle(a)} />
                    <span>{a}</span>
                  </label>
                ))}
                <Button variant="outline" type="button" onClick={addScope}
                  disabled={!pattern.trim() || actions.length === 0}>Add</Button>
              </Inline>
            </div>

            <div>
              <label className="block text-sm font-medium">Expires on<span className="text-destructive">*</span></label>
              <Input type="date" value={expiresOn} min={dateStr(minDate)} max={dateStr(maxDate)}
                onChange={(e) => setExpiresOn(e.target.value)} required />
              <p className="mt-1.5 text-sm text-muted-foreground">Tokens expire after at most one year.</p>
            </div>

            {error && <Alert>{error}</Alert>}
            <Inline className="pt-1">
              <Button type="submit" disabled={!valid}>Create</Button>
              <Button variant="outline" type="button" onClick={() => navigate(forUserId ? { to: "/users/$id", params: { id: String(forUserId) } } : { to: "/tokens" })}>Cancel</Button>
            </Inline>
          </form>
        </PanelBody>
      </Panel>
    </>
  );
}
