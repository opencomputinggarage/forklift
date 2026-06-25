import { FormEvent, useEffect, useState } from "react";
import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";
import { api } from "@/api";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Combobox,
  ComboboxChip,
  ComboboxChips,
  ComboboxChipsInput,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  useComboboxAnchor,
} from "@/components/ui/combobox";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Plus, X } from "lucide-react";

export const Route = createFileRoute("/workspace/tokens/new")({
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

// Token creation page. Reached from the New token button on /workspace/tokens
// (self-service for the current user) or from a user's detail page at
// /access/users/:id/workspace/tokens/new (admin creating a token for that user). The presence of
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
  const [actionSearch, setActionSearch] = useState("");
  const actionAnchorRef = useComboboxAnchor();
  const actionOptions = ACTIONS.filter((action) => action.includes(actionSearch.trim().toLowerCase()));

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

  const addScope = () => {
    if (!pattern.trim() || actions.length === 0) return;
    setScopes((cur) => [...cur, { repo_pattern: pattern.trim(), actions: [...actions] }]);
    setPattern("");
    setActions(["read"]);
    setActionSearch("");
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
        <PageHeader title="Token created" />
        <PageDescription>
          Copy this token now. It will not be shown again.
        </PageDescription>
        <Card size="sm" className="mb-4 max-w-[40rem]">
          <CardContent>
            <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap items-stretch max-sm:flex-col">
              <div className="min-h-8 flex-1 overflow-x-auto rounded-lg border border-border bg-muted px-3 py-2 font-mono text-xs">
                {created}
              </div>
              <Button variant="outline" type="button" onClick={copy}>
                {copied ? "Copied" : "Copy"}
              </Button>
            </div>
            <Button className="mt-5" onClick={() => navigate(forUserId ? { to: "/access/users/$id", params: { id: String(forUserId) } } : { to: "/workspace/tokens" })}>Done</Button>
          </CardContent>
        </Card>
      </>
    );
  }

  return (
    <>
      <PageHeader title={forUserId !== null ? "Create token for user" : "Create token"} />
      <PageDescription>
        Issue a scoped access token with an expiration date and repository permissions.
      </PageDescription>

      <Card size="sm" className="mb-4 max-w-[44rem]">
        <CardContent>
          <form onSubmit={submit} className="space-y-5">
            <FieldGroup className="gap-4">
              <Field>
                <FieldLabel htmlFor="token-name">
                  Token name<span className="text-destructive">*</span>
                </FieldLabel>
                <Input
                  id="token-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="ci"
                  autoFocus
                  required
                  pattern="[A-Za-z0-9_-]{1,64}"
                  title="Letters, digits, '-' and '_' only (max 64 characters)"
                />
                <FieldDescription>Letters, digits, dash and underscore only.</FieldDescription>
              </Field>

              <Field>
                <FieldLabel htmlFor="token-description">
                  Token description<span className="text-destructive">*</span>
                </FieldLabel>
                <Input
                  id="token-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="What this token is used for"
                  required
                />
              </Field>

              <Field>
                <FieldLabel htmlFor="expires-on">
                  Expires on<span className="text-destructive">*</span>
                </FieldLabel>
                <Input
                  id="expires-on"
                  type="date"
                  value={expiresOn}
                  min={dateStr(minDate)}
                  max={dateStr(maxDate)}
                  onChange={(e) => setExpiresOn(e.target.value)}
                  required
                />
                <FieldDescription>Tokens expire after at most one year.</FieldDescription>
              </Field>
            </FieldGroup>

            <div className="space-y-3 border-t border-border pt-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="m-0 text-sm font-semibold">Permissions</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Add repository patterns and allowed token actions.
                  </p>
                </div>
                <Badge className="mt-0.5">{scopes.length}</Badge>
              </div>

              <div className="min-h-10 rounded-lg border border-border bg-muted/20 p-2">
                <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap flex-wrap gap-1.5">
                {scopes.map((s, i) => (
                  <Badge key={`${s.repo_pattern}-${i}`} className="font-mono">
                    {s.repo_pattern}: {s.actions.join(",")}
                    <Button
                      className="-mr-1 ml-1 size-4 rounded-full text-muted-foreground hover:bg-background/40 hover:text-foreground"
                      size="icon-xs"
                      variant="ghost"
                      type="button"
                      title="Remove permission"
                      onClick={() => setScopes((cur) => cur.filter((_, j) => j !== i))}
                    >
                      <X className="size-3" aria-hidden="true" />
                    </Button>
                  </Badge>
                ))}
                  {scopes.length === 0 && (
                    <span className="px-1 text-sm text-muted-foreground">No permissions added.</span>
                  )}
                </div>
              </div>

              <div className="rounded-lg border border-border/80 bg-background/40 p-3">
                <FieldGroup className="gap-3">
                  <Field>
                    <FieldLabel>Repository pattern</FieldLabel>
                    <Combobox
                      items={repoOptions}
                      inputValue={pattern}
                      value={repoOptions.includes(pattern) ? pattern : null}
                      onInputValueChange={setPattern}
                      onValueChange={(next) => {
                        if (typeof next === "string") setPattern(next);
                      }}
                    >
                      <ComboboxInput placeholder="repo pattern (* or maven-*)" className="w-full" />
                      <ComboboxContent>
                        <ComboboxEmpty>No repositories found.</ComboboxEmpty>
                        <ComboboxList>
                          {repoOptions.map((option) => (
                            <ComboboxItem key={option} value={option}>
                              <span className="min-w-0 truncate">
                                {option}
                                {repoTypes[option] && (
                                  <span className="ml-2 text-xs text-muted-foreground">
                                    {repoTypes[option]}
                                  </span>
                                )}
                              </span>
                            </ComboboxItem>
                          ))}
                        </ComboboxList>
                      </ComboboxContent>
                    </Combobox>
                  </Field>

                  <Field>
                    <FieldLabel>Actions</FieldLabel>
                    <Combobox
                      multiple
                      items={actionOptions}
                      inputValue={actionSearch}
                      value={actions}
                      onInputValueChange={setActionSearch}
                      onValueChange={(next) => {
                        setActions(next);
                        setActionSearch("");
                      }}
                    >
                      <ComboboxChips ref={actionAnchorRef} className="w-full">
                        {actions.map((action) => (
                          <ComboboxChip key={action}>{action}</ComboboxChip>
                        ))}
                        <ComboboxChipsInput
                          placeholder={actions.length ? "Add action" : "Select actions"}
                        />
                      </ComboboxChips>
                      <ComboboxContent anchor={actionAnchorRef}>
                        <ComboboxEmpty>No actions found.</ComboboxEmpty>
                        <ComboboxList>
                          {actionOptions.map((action) => (
                            <ComboboxItem key={action} value={action}>
                              {action}
                            </ComboboxItem>
                          ))}
                        </ComboboxList>
                      </ComboboxContent>
                    </Combobox>
                  </Field>

                  <div className="flex justify-end">
                    <Button
                      variant="outline"
                      type="button"
                      onClick={addScope}
                      disabled={!pattern.trim() || actions.length === 0}
                    >
                      <Plus data-icon="inline-start" />
                      Add permission
                    </Button>
                  </div>
                </FieldGroup>
              </div>
            </div>

            {error && <Alert>{error}</Alert>}
            <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap border-t border-border pt-4">
              <Button type="submit" disabled={!valid}>Create token</Button>
              <Button variant="outline" type="button" onClick={() => navigate(forUserId ? { to: "/access/users/$id", params: { id: String(forUserId) } } : { to: "/workspace/tokens" })}>Cancel</Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </>
  );
}
