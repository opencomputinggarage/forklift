import { FormEvent, useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate } from "@tanstack/react-router";
import { api } from "../../api";
import { useAuth } from "../../authContext";
import { Combobox } from "../../components/combobox";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Inline, Panel, PanelBody } from "@/components/app-ui/page";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/roles/new")({
  component: RoleNewRoute,
});

function RoleNewRoute() {
  const { me } = useAuth();
  return me.admin ? <RoleNew /> : <Navigate to="/repositories" replace />;
}

const ACTIONS = ["read", "write", "delete", "approve", "admin"];

interface Permission {
  repo_pattern: string;
  actions: string[];
}

// Admin-only role creation, reached from the Create button on /roles.
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

  const toggle = (a: string) =>
    setActions((cur) => cur.includes(a) ? cur.filter((x) => x !== a) : [...cur, a]);

  const addPermission = () => {
    if (!pattern.trim() || actions.length === 0) return;
    setPermissions((cur) => [...cur, { repo_pattern: pattern.trim(), actions: [...actions] }]);
    setPattern("");
    setActions(["read"]);
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
      navigate({ to: "/roles" });
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <>
      <h1>Create role</h1>
      <Panel className="max-w-[35rem]">
        <PanelBody>
          <form onSubmit={submit} className="space-y-4">
            <label className="block text-sm font-medium">Role name<span className="text-destructive">*</span></label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="maven-readers" autoFocus required
              pattern="[A-Za-z0-9_-]{1,64}" title="Letters, digits, '-' and '_' only (max 64 characters)" />

            <label className="block text-sm font-medium">Description</label>
            <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="optional" />

            <div>
              <label className="block text-sm font-medium">Permissions</label>
              <Inline className="mt-2 flex-wrap gap-1.5">
                {permissions.map((p, i) => (
                  <Badge key={i} className="font-mono">
                    {p.repo_pattern}: {p.actions.join(",")}
                    <button className="ml-1.5 cursor-pointer" type="button" title="Remove permission"
                      onClick={() => setPermissions((cur) => cur.filter((_, j) => j !== i))}>x</button>
                  </Badge>
                ))}
                {permissions.length === 0 && <span className="text-sm text-muted-foreground">none yet (optional)</span>}
              </Inline>
              <Inline className="mt-2 flex-wrap gap-2">
                <Combobox style={{ width: 200 }} value={pattern} onChange={setPattern}
                  options={repoOptions} hints={repoTypes} placeholder="repo pattern (* or maven-*)" />
                {ACTIONS.map((a) => (
                  <label key={a} className="inline-flex items-center gap-1.5 text-xs">
                    <Checkbox checked={actions.includes(a)} onCheckedChange={() => toggle(a)} />
                    <span>{a}</span>
                  </label>
                ))}
                <Button variant="outline" type="button" onClick={addPermission}
                  disabled={!pattern.trim() || actions.length === 0}>Add</Button>
              </Inline>
            </div>
            <p className="text-sm text-muted-foreground">Permissions are optional here and can also be granted on the Roles page later. Assign the role to users on the Users page.</p>

            {error && <Alert>{error}</Alert>}
            <Inline className="pt-1">
              <Button type="submit" disabled={!name.trim()}>Create</Button>
              <Button variant="outline" type="button" onClick={() => navigate({ to: "/roles" })}>Cancel</Button>
            </Inline>
          </form>
        </PanelBody>
      </Panel>
    </>
  );
}
