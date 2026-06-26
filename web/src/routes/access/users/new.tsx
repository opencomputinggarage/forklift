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
import { useTranslation } from "@/lib/i18n";

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
  const { t } = useTranslation();
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
      setError(t("user.password-mismatch"));
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
      <PageHeader title={t("user.create-local")} />
      <PageDescription>
        {t("user.new-description")}
      </PageDescription>

      <Card size="sm" className="mb-4 max-w-[44rem]">
        <CardContent>
          <form onSubmit={submit} className="space-y-5">
            <FieldGroup className="gap-4">
              <Field>
                <FieldLabel htmlFor="username">
                  {t("common.username")}<span className="text-destructive">*</span>
                </FieldLabel>
                <Input
                  id="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoFocus
                  required
                  pattern="[A-Za-z0-9_-]{1,64}"
                  title={t("common.name-rule-64")}
                />
                <FieldDescription>{t("common.name-rule")}</FieldDescription>
              </Field>

              <Field>
                <FieldLabel htmlFor="email">{t("common.email")}</FieldLabel>
                <Input
                  id="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder={t("common.optional")}
                />
                <FieldDescription>
                  {t("user.oidc-note")}
                </FieldDescription>
              </Field>

              <Field>
                <FieldLabel htmlFor="role">{t("common.role")}</FieldLabel>
                <Select
                  value={roleId}
                  onChange={setRoleId}
                  placeholder={t("user.no-role-placeholder")}
                  options={roles.map((r) => ({
                    value: String(r.id),
                    label: r.name,
                    description: r.description || undefined,
                  }))}
                />
                <FieldDescription>
                  {t("user.no-role-note")}
                </FieldDescription>
              </Field>
            </FieldGroup>

            <div className="space-y-3 border-t border-border pt-4">
              <div>
                <h2 className="m-0 text-sm font-semibold">{t("common.password")}</h2>
                <p className="mt-1 text-sm text-muted-foreground">
                  {t("user.initial-password-note")}
                </p>
              </div>

              <FieldGroup className="gap-4">
                <Field>
                  <FieldLabel htmlFor="password">
                    {t("common.password")}<span className="text-destructive">*</span>
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
                      aria-label={show ? t("common.hide-password") : t("common.show-password")}
                    >
                      {show ? t("common.hide") : t("common.show")}
                    </Button>
                  </div>
                </Field>

                <Field>
                  <FieldLabel htmlFor="confirm-password">
                    {t("common.confirm-password")}<span className="text-destructive">*</span>
                  </FieldLabel>
                  <Input
                    id="confirm-password"
                    type={show ? "text" : "password"}
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    required
                    aria-invalid={mismatch}
                  />
                  {mismatch && <Alert>{t("user.password-mismatch")}</Alert>}
                </Field>
              </FieldGroup>
            </div>

            {error && <Alert>{error}</Alert>}

            <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap border-t border-border pt-4">
              <Button type="submit" disabled={!canSubmit}>{t("user.create")}</Button>
              <Button variant="outline" type="button" onClick={() => navigate({ to: "/access/users" })}>{t("common.cancel")}</Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </>
  );
}
