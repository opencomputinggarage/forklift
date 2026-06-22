import { FormEvent, useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate } from "@tanstack/react-router";
import { api, Role } from "../../api";
import { useAuth } from "../../authContext";
import { Select } from "@/components/app-ui/select";
import { Alert } from "@/components/app-ui/alert";
import { Inline, Panel, PanelBody } from "@/components/app-ui/page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/users/new")({
  component: UserNewRoute,
});

function UserNewRoute() {
  const { me } = useAuth();
  return me.admin ? <UserNew /> : <Navigate to="/repositories" replace />;
}

// Admin-only local user creation, reached from the Create button on /users.
// OIDC users are never created here; they appear at first SSO login.
export function UserNew() {
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [show, setShow] = useState(false);
  const [email, setEmail] = useState("");
  const [roleId, setRoleId] = useState("");
  const [roles, setRoles] = useState<Role[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    api.listRoles().then(setRoles).catch(() => setRoles([]));
  }, []);

  const mismatch = confirm.length > 0 && password !== confirm;
  const canSubmit = username.trim() && password && password === confirm;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    if (password !== confirm) {
      setError("Passwords do not match.");
      return;
    }
    try {
      await api.createUser({
        username, password, email: email || undefined,
        role_ids: roleId ? [Number(roleId)] : undefined,
      });
      navigate({ to: "/users" });
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <>
      <h1>Create local user</h1>
      <Panel className="max-w-[35rem]">
        <PanelBody>
          <form onSubmit={submit} className="space-y-4">
            <label className="block text-sm font-medium">Username<span className="text-destructive">*</span></label>
            <Input value={username} onChange={(e) => setUsername(e.target.value)} autoFocus required
              pattern="[A-Za-z0-9_-]{1,64}" title="Letters, digits, '-' and '_' only (max 64 characters)" />

            <div>
              <label className="block text-sm font-medium">Password<span className="text-destructive">*</span></label>
              <Inline>
                <Input type={show ? "text" : "password"} value={password}
                  onChange={(e) => setPassword(e.target.value)} required />
                <Button type="button" variant="outline"
                  onClick={() => setShow((s) => !s)}
                  aria-label={show ? "Hide password" : "Show password"}>
                  {show ? "Hide" : "Show"}
                </Button>
              </Inline>
            </div>

            <div>
              <label className="block text-sm font-medium">Confirm password<span className="text-destructive">*</span></label>
              <Inline>
                <Input type={show ? "text" : "password"} value={confirm}
                  onChange={(e) => setConfirm(e.target.value)} required
                  aria-invalid={mismatch} />
                <Button type="button" variant="outline"
                  onClick={() => setShow((s) => !s)}
                  aria-label={show ? "Hide password" : "Show password"}>
                  {show ? "Hide" : "Show"}
                </Button>
              </Inline>
              {mismatch && <Alert className="mt-2">Passwords do not match.</Alert>}
            </div>

            <div>
              <label className="block text-sm font-medium">Role</label>
              <Select value={roleId} onChange={setRoleId} placeholder="no role"
                options={roles.map((r) => ({ value: String(r.id), label: r.name, description: r.description || undefined }))} />
              <p className="mt-1.5 text-sm text-muted-foreground">A local user with no role cannot access any repository until one is assigned.</p>
            </div>

            <div>
              <label className="block text-sm font-medium">Email</label>
              <Input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="optional" />
              <p className="mt-1.5 text-sm text-muted-foreground">OIDC users are created automatically at first login; their access comes from group mappings.</p>
            </div>
            {error && <Alert>{error}</Alert>}
            <Inline className="pt-1">
              <Button type="submit" disabled={!canSubmit}>Create</Button>
              <Button variant="outline" type="button" onClick={() => navigate({ to: "/users" })}>Cancel</Button>
            </Inline>
          </form>
        </PanelBody>
      </Panel>
    </>
  );
}
