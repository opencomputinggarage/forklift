import { FormEvent, useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate } from "@tanstack/react-router";
import { api, Role } from "@/api";
import { useAuth } from "@/authContext";
import { Select } from "@/components/app-ui/select";
import { Alert } from "@/components/app-ui/alert";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/access/users/new")({
  component: UserNewRoute,
});

function UserNewRoute() {
  const { me } = useAuth();
  return me.admin ? <UserNew /> : <Navigate to="/workspace/repositories" replace />;
}

// Admin-only local user creation, reached from the Create button on /access/users.
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
      navigate({ to: "/access/users" });
    } catch (err) {
      setError((err as Error).message);
    }
  };

  return (
    <>
      <PageHeader title="Create local user" />
      <PageDescription>
        Create a local account and optionally assign an initial role.
      </PageDescription>

      <Card size="sm" className="mb-4 max-w-[44rem]">
        <CardContent>
          <form onSubmit={submit} className="space-y-5">
            <FieldGroup className="gap-4">
              <Field>
                <FieldLabel htmlFor="username">
                  Username<span className="text-destructive">*</span>
                </FieldLabel>
                <Input
                  id="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoFocus
                  required
                  pattern="[A-Za-z0-9_-]{1,64}"
                  title="Letters, digits, '-' and '_' only (max 64 characters)"
                />
                <FieldDescription>Letters, digits, dash and underscore only.</FieldDescription>
              </Field>

              <Field>
                <FieldLabel htmlFor="email">Email</FieldLabel>
                <Input
                  id="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="optional"
                />
                <FieldDescription>
                  OIDC users are created automatically at first login.
                </FieldDescription>
              </Field>

              <Field>
                <FieldLabel htmlFor="role">Role</FieldLabel>
                <Select
                  value={roleId}
                  onChange={setRoleId}
                  placeholder="no role"
                  options={roles.map((r) => ({
                    value: String(r.id),
                    label: r.name,
                    description: r.description || undefined,
                  }))}
                />
                <FieldDescription>
                  A user with no role cannot access repositories until one is assigned.
                </FieldDescription>
              </Field>
            </FieldGroup>

            <div className="space-y-3 border-t border-border pt-4">
              <div>
                <h2 className="m-0 text-sm font-semibold">Password</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  Set an initial password for this local account.
                </p>
              </div>

              <FieldGroup className="gap-4">
                <Field>
                  <FieldLabel htmlFor="password">
                    Password<span className="text-destructive">*</span>
                  </FieldLabel>
                  <div className="flex min-w-0 items-stretch gap-2 max-sm:flex-wrap max-sm:flex-col">
                    <Input
                      id="password"
                      type={show ? "text" : "password"}
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      required
                    />
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => setShow((s) => !s)}
                      aria-label={show ? "Hide password" : "Show password"}
                    >
                      {show ? "Hide" : "Show"}
                    </Button>
                  </div>
                </Field>

                <Field>
                  <FieldLabel htmlFor="confirm-password">
                    Confirm password<span className="text-destructive">*</span>
                  </FieldLabel>
                  <Input
                    id="confirm-password"
                    type={show ? "text" : "password"}
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    required
                    aria-invalid={mismatch}
                  />
                  {mismatch && <Alert>Passwords do not match.</Alert>}
                </Field>
              </FieldGroup>
            </div>

            {error && <Alert>{error}</Alert>}

            <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap border-t border-border pt-4">
              <Button type="submit" disabled={!canSubmit}>Create user</Button>
              <Button variant="outline" type="button" onClick={() => navigate({ to: "/access/users" })}>Cancel</Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </>
  );
}
