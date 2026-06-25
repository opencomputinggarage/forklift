import { FormEvent, useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate } from "@tanstack/react-router";
import { api } from "@/api";
import { useAuth } from "@/authContext";
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

export const Route = createFileRoute("/access/roles/new")({
  component: RoleNewRoute,
});

function RoleNewRoute() {
  const { me } = useAuth();
  return me.admin ? <RoleNew /> : <Navigate to="/workspace/repositories" replace />;
}

const ACTIONS = ["read", "write", "delete", "approve", "admin"];

interface Permission {
  repo_pattern: string;
  actions: string[];
}

// Admin-only role creation, reached from the Create button on /access/roles.
// Permissions can be granted here at creation, or added later on the Roles page.
export function RoleNew() {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [error, setError] = useState("");

  // Permission add-row state.
  const [pattern, setPattern] = useState("");
  const [actions, setActions] = useState<string[]>(["read"]);
  const [actionSearch, setActionSearch] = useState("");
  const actionAnchorRef = useComboboxAnchor();
  const actionOptions = ACTIONS.filter((action) => action.includes(actionSearch.trim().toLowerCase()));

  // Repository names for pattern autocomplete; "*" (all) is offered first.
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

  const addPermission = () => {
    if (!pattern.trim() || actions.length === 0) return;
    setPermissions((cur) => [...cur, { repo_pattern: pattern.trim(), actions: [...actions] }]);
    setPattern("");
    setActions(["read"]);
    setActionSearch("");
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    try {
      await api.createRole({
        name,
        description: description || undefined,
        permissions: permissions.length ? permissions : undefined,
      });
      navigate({ to: "/access/roles" });
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <>
      <PageHeader title="Create role" />
      <PageDescription>
        Define a reusable access profile and optional repository permissions.
      </PageDescription>

      <Card size="sm" className="mb-4 max-w-[44rem]">
        <CardContent>
          <form onSubmit={submit} className="space-y-5">
            <FieldGroup className="gap-4">
              <Field>
                <FieldLabel htmlFor="role-name">
                  Role name<span className="text-destructive">*</span>
                </FieldLabel>
                <Input
                  id="role-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="maven-readers"
                  autoFocus
                  required
                  pattern="[A-Za-z0-9_-]{1,64}"
                  title="Letters, digits, '-' and '_' only (max 64 characters)"
                />
                <FieldDescription>Letters, digits, dash and underscore only.</FieldDescription>
              </Field>

              <Field>
                <FieldLabel htmlFor="role-description">Description</FieldLabel>
                <Input
                  id="role-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="optional"
                />
              </Field>
            </FieldGroup>

            <div className="space-y-3 border-t border-border pt-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="m-0 text-sm font-semibold">Permissions</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Add repository patterns and allowed actions. Wildcards are accepted.
                  </p>
                </div>
                <Badge className="mt-0.5">{permissions.length}</Badge>
              </div>

              <div className="min-h-10 rounded-lg border border-border bg-muted/20 p-2">
                <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                  {permissions.map((p, i) => (
                    <Badge key={`${p.repo_pattern}-${i}`} className="font-mono">
                      {p.repo_pattern}: {p.actions.join(",")}
                      <Button
                        className="-mr-1 ml-1 size-4 rounded-full text-muted-foreground hover:bg-background/40 hover:text-foreground"
                        size="icon-xs"
                        variant="ghost"
                        type="button"
                        title="Remove permission"
                        onClick={() => setPermissions((cur) => cur.filter((_, j) => j !== i))}
                      >
                        <X className="size-3" aria-hidden="true" />
                      </Button>
                    </Badge>
                  ))}
                  {permissions.length === 0 && (
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
                      onClick={addPermission}
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
              <Button type="submit" disabled={!name.trim()}>
                Create role
              </Button>
              <Button variant="outline" type="button" onClick={() => navigate({ to: "/access/roles" })}>
                Cancel
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </>
  );
}
