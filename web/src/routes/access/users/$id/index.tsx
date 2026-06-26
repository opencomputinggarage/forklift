import { ReactNode, useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate, useParams } from "@tanstack/react-router";
import { Eye, EyeOff, LockKeyhole, X } from "lucide-react";
import { api, Me, Role, Token, User } from "@/api";
import { useAuth } from "@/authContext";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { Select } from "@/components/app-ui/select";
import { Alert } from "@/components/app-ui/alert";
import { PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrap,
} from "@/components/app-ui/table";
import { Badge } from "@/components/app-ui/badge";
import { StateBadge } from "@/components/app-ui/status-badge";
import { Button } from "@/components/ui/button";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/access/users/$id/")({
  component: UserModifyRoute,
});

function UserModifyRoute() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <UserModify me={me} /> : <Navigate to="/workspace/repositories" replace />;
}

interface Scope {
  repo_pattern: string;
  actions: string[];
}

function parseScopes(json: string): Scope[] {
  try {
    const v = JSON.parse(json);
    return Array.isArray(v) ? v : [];
  } catch {
    return [];
  }
}

// Per-user modify page: role mapping, password reset, enable/disable, and the
// danger zone (delete). The Users list is read-only; all edits happen here.
export function UserModify({ me }: { me: Me }) {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id?: string };
  const navigate = useNavigate();
  const userId = Number(id);
  const [user, setUser] = useState<User | null>(null);
  const [roles, setRoles] = useState<Role[]>([]);
  const [tokens, setTokens] = useState<Token[]>([]);
  const [error, setError] = useState("");

  const load = () =>
    Promise.all([api.listUsers(), api.listRoles(), api.listUserTokens(userId)])
      .then(([users, rs, ts]) => {
        const u = users.find((x) => x.id === userId) ?? null;
        setUser(u);
        setRoles(rs);
        setTokens(ts);
        if (!u) setError("User not found.");
      })
      .catch((e) => setError(e.message));
  useEffect(() => { load(); /* eslint-disable-next-line */ }, [userId]);

  if (error && !user) return <Alert className="my-2.5">{error}</Alert>;
  if (!user) return <div className="text-sm text-muted-foreground">{t("common.loading")}</div>;

  const self = user.username === me.username;

  const run = (p: Promise<unknown>) => {
    setError("");
    p.then(load).catch((e) => setError((e as Error).message));
  };

  return (
    <>
      <PageHeader
        title={
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <span className="min-w-0 truncate">{user.username}</span>
            <Badge>{user.source}</Badge>
            {self && <Badge>{t("common.you")}</Badge>}
          </div>
        }
        actions={
          <Button variant="outline" onClick={() => navigate({ to: "/access/users" })}>
            {t("user.back")}
          </Button>
        }
      />
      {error && <Alert className="mb-4">{error}</Alert>}

      <AccountPanel user={user} />
      <RolesPanel user={user} roles={roles} run={run} canWrite={!!me.admin} />
      <TokensPanel user={user} tokens={tokens} canWrite={!!me.admin} run={run} />
      {me.admin && user.source === "local" && <PasswordPanel user={user} onError={setError} />}
      {me.admin && user.source === "local" && <LockoutPanel user={user} run={run} />}
      {me.admin && <StatusPanel user={user} self={self} run={run} />}
      {me.admin && <DangerPanel user={user} self={self} onDeleted={() => navigate({ to: "/access/users" })} onError={setError} />}
    </>
  );
}

// AccountPanel shows the identity fields read-only. Username and email are owned
// by the identity provider (OIDC) or set at creation (local), so they are not
// editable here — only displayed.
function AccountPanel({ user }: { user: User }) {
  const { t } = useTranslation();
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
        <h2 className="m-0 mb-4 text-base font-semibold">{t("common.account")}</h2>
        <FieldGroup className="grid gap-4 sm:grid-cols-2">
          <ReadOnlyField label={t("common.username")} value={user.username} />
          <ReadOnlyField label={t("common.email")} value={user.email || "—"} />
          <ReadOnlyField label={t("common.created")} value={user.created_at ? new Date(user.created_at).toLocaleString() : "—"} />
          <ReadOnlyField label={t("common.last-login")} value={user.last_login_at ? new Date(user.last_login_at).toLocaleString() : t("common.never")} />
        </FieldGroup>
      </CardContent>
    </Card>
  );
}

function ReadOnlyField({ label, value }: { label: string; value: string }) {
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      <Input value={value} readOnly className="bg-muted/20 text-muted-foreground" />
    </Field>
  );
}

function RolesPanel({ user, roles, run, canWrite }: { user: User; roles: Role[]; run: (p: Promise<unknown>) => void; canWrite: boolean }) {
  const { t } = useTranslation();
  const [selected, setSelected] = useState("");
  const assignable = roles.filter((r) => !user.roles.some((ur) => ur.id === r.id));

  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">{t("common.roles")}</h2>
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        {user.roles.map((r) => (
          <Badge key={r.id} className="gap-1">
            <span>{r.name}</span>
            {canWrite && (
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                className="-mr-1 size-4 rounded-full text-muted-foreground hover:bg-background/40 hover:text-foreground"
                title={t("user.remove-role")}
                onClick={() => run(api.removeRole(user.id, r.id))}
              >
                <X className="size-3" aria-hidden="true" />
                <span className="sr-only">{t("user.remove-role")}</span>
              </Button>
            )}
          </Badge>
        ))}
        {user.roles.length === 0 && <span className="text-sm text-muted-foreground">{t("user.no-roles")}</span>}
      </div>
      {canWrite && assignable.length > 0 && (
        <div className="flex min-w-0 items-center gap-2 mt-4 max-sm:flex-wrap items-stretch max-sm:flex-col">
          <Select value={selected} onChange={setSelected} placeholder={t("user.add-role-placeholder")}
            options={assignable.map((r) => ({ value: String(r.id), label: r.name, description: r.description || undefined }))} />
          <Button variant="outline" type="button" disabled={!selected}
            onClick={() => { run(api.assignRole(user.id, Number(selected))); setSelected(""); }}>
            {t("common.add")}
          </Button>
        </div>
      )}
      </CardContent>
    </Card>
  );
}

// TokensPanel shows the user's personal access tokens. Admins can create
// (via the New token page) and revoke them; auditors see the list read-only.
// Token scopes only ever narrow the user's own role permissions, so issuing a
// token here cannot grant access the user does not already have.
function TokensPanel({ user, tokens, canWrite, run }: {
  user: User; tokens: Token[]; canWrite: boolean; run: (p: Promise<unknown>) => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [revokeId, setRevokeId] = useState<number | null>(null);

  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <div className="mb-4 flex items-start justify-between gap-3 max-sm:flex-col max-sm:items-stretch">
        <h2 className="m-0 text-base font-semibold">
          {t("token.title")} <span className="text-xs font-normal text-muted-foreground">{t("token.subtitle")}</span>
        </h2>
        {canWrite && (
          <Button
            onClick={() => navigate({ to: "/access/users/$id/tokens/new", params: { id: String(user.id) } })}
          >
            {t("token.new")}
          </Button>
        )}
      </div>
      <TableWrap>
        <Table>
        <TableHeader><TableRow><TableHead>{t("common.name")}</TableHead><TableHead>{t("common.description")}</TableHead><TableHead>{t("common.permissions")}</TableHead><TableHead>{t("common.created")}</TableHead><TableHead>{t("common.expires")}</TableHead><TableHead>{t("common.last-used")}</TableHead>{canWrite && <TableHead></TableHead>}</TableRow></TableHeader>
        <TableBody>
          {tokens.map((tok) => (
            <TableRow key={tok.id}>
              <TableCell>{tok.name}</TableCell>
              <TableCell className="text-muted-foreground">{tok.description}</TableCell>
              <TableCell>
                {parseScopes(tok.scopes_json).map((s, i) => (
                  <Badge key={i} className="mr-1 font-mono">
                    {s.repo_pattern}: {s.actions.join(",")}
                  </Badge>
                ))}
              </TableCell>
              <TableCell className="text-muted-foreground">{tok.created_at?.slice(0, 10)}</TableCell>
              <TableCell className="text-muted-foreground">{tok.expires_at ? tok.expires_at.slice(0, 10) : t("common.never")}</TableCell>
              <TableCell className="text-muted-foreground">{tok.last_used_at ? tok.last_used_at.slice(0, 10) : t("common.never")}</TableCell>
              {canWrite && <TableCell><Button variant="destructive" onClick={() => setRevokeId(tok.id)}>{t("token.revoke")}</Button></TableCell>}
            </TableRow>
          ))}
          {tokens.length === 0 && <TableRow><TableCell colSpan={canWrite ? 7 : 6} className="text-muted-foreground">{t("token.empty")}</TableCell></TableRow>}
        </TableBody>
        </Table>
      </TableWrap>
      {!canWrite && <p className="mb-0 mt-3 text-sm text-muted-foreground">{t("token.readonly-note")}</p>}
      <ConfirmModal
        open={revokeId !== null}
        title={t("token.revoke-confirm-title")}
        message={t("token.revoke-confirm-message")}
        confirmLabel={t("token.revoke")}
        danger
        onConfirm={() => { if (revokeId !== null) run(api.deleteUserToken(user.id, revokeId)); setRevokeId(null); }}
        onCancel={() => setRevokeId(null)}
      />
      </CardContent>
    </Card>
  );
}

function PasswordPanel({ user, onError }: { user: User; onError: (e: string) => void }) {
  const { t } = useTranslation();
  const [password, setPassword] = useState("");
  const [show, setShow] = useState(false);
  const [saved, setSaved] = useState(false);

  const reset = async () => {
    onError("");
    setSaved(false);
    try {
      await api.updateUser(user.id, { password });
      setPassword("");
      setSaved(true);
    } catch (e) {
      onError((e as Error).message);
    }
  };

  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">{t("common.password")}</h2>
      <Field>
        <FieldLabel>{t("common.new-password")}</FieldLabel>
        <div className="relative">
        <Input className="pr-16" type={show ? "text" : "password"} value={password}
          onChange={(e) => { setPassword(e.target.value); setSaved(false); }} />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="absolute right-0 top-0 h-full rounded-l-none text-muted-foreground"
          onClick={() => setShow((s) => !s)}
          aria-label={show ? t("common.hide-password") : t("common.show-password")}
        >
          {show ? <EyeOff className="size-4" aria-hidden="true" /> : <Eye className="size-4" aria-hidden="true" />}
        </Button>
        </div>
      </Field>
      <div className="flex min-w-0 items-center gap-2 mt-4 max-sm:flex-wrap max-sm:flex-col max-sm:items-stretch">
        <Button type="button" disabled={!password} onClick={reset}>{t("user.reset-password")}</Button>
        {saved && <span className="text-sm text-muted-foreground">{t("user.password-updated")}</span>}
      </div>
      </CardContent>
    </Card>
  );
}

// LockoutPanel toggles failed-password lockout for a local account and unlocks
// it after a lockout. The default admin is protected: the toggle is disabled so
// it can never be locked out of the only guaranteed admin account.
function LockoutPanel({ user, run }: { user: User; run: (p: Promise<unknown>) => void }) {
  const { t } = useTranslation();
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-3 text-base font-semibold">{t("common.account-lockout")}</h2>
      <p className="text-sm leading-relaxed text-muted-foreground">
        {t("user.lockout-note")} {user.protected && t("user.lockout-protected-note")}
      </p>
      <label className="inline-flex items-center gap-2 text-sm">
        <Switch
          checked={user.lockout_enabled}
          disabled={user.protected}
          onCheckedChange={(v) => run(api.updateUser(user.id, { lockout_enabled: v }))}
          aria-label={user.lockout_enabled ? t("user.lockout-enabled") : t("user.lockout-disabled")}
        />
        <span>{user.lockout_enabled ? t("user.lockout-enabled") : t("user.lockout-disabled")}</span>
      </label>
      {user.locked && (
        <div className="flex min-w-0 items-center gap-2 mt-4 max-sm:flex-wrap max-sm:flex-col max-sm:items-stretch">
          <StateBadge state="locked">{t("common.locked")}</StateBadge>
          <Button type="button"
            onClick={() => run(api.updateUser(user.id, { unlock: true }))}>
            {t("user.unlock")}
          </Button>
        </div>
      )}
      </CardContent>
    </Card>
  );
}

// LockNote renders an accent-bordered callout with a lock icon, matching the
// managed-role notice. Used to explain why an edit control is locked.
function LockNote({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="mt-4 rounded-lg border border-primary/70 bg-primary/5 p-4">
      <h2 className="mb-2 flex items-center gap-2 text-[15px] font-semibold">
        <LockKeyhole className="size-4 text-primary" aria-hidden="true" />
        {title}
      </h2>
      <p className="m-0 text-sm leading-relaxed text-muted-foreground">{children}</p>
    </div>
  );
}

function StatusPanel({ user, self, run }: { user: User; self: boolean; run: (p: Promise<unknown>) => void }) {
  const { t } = useTranslation();
  const lockedFromEditing = self || user.protected;
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-3 text-base font-semibold">{t("common.status")}</h2>
      <p className="text-sm leading-relaxed text-muted-foreground">
        An <strong>{t("common.status.active")}</strong> {t("user.status-desc-1")} <strong>{t("user.status-desc-disabling")}</strong> {t("user.status-desc-2")}
      </p>
      <label className="inline-flex items-center gap-2 text-sm">
        <Switch
          checked={!user.disabled}
          disabled={lockedFromEditing}
          onCheckedChange={(v) => run(api.updateUser(user.id, { disabled: !v }))}
          aria-label={user.disabled ? t("user.account-disabled") : t("user.account-active")}
        />
        <span>{user.disabled ? t("user.account-disabled") : t("user.account-active")}</span>
      </label>
      {user.protected ? (
        <LockNote title={t("user.status-locked")}>
          {t("user.protected-note")}
        </LockNote>
      ) : self && (
        <LockNote title={t("user.status-locked")}>
          {t("user.self-note")}
        </LockNote>
      )}
      {user.locked && (
        <p className="mb-0 mt-3">
          <StateBadge state="locked">{t("common.locked")}</StateBadge>
          <span className="ml-2 text-sm text-muted-foreground">
            {t("user.locked-note")}
          </span>
        </p>
      )}
      </CardContent>
    </Card>
  );
}

function DangerPanel({ user, self, onDeleted, onError }: {
  user: User; self: boolean; onDeleted: () => void; onError: (e: string) => void;
}) {
  const { t } = useTranslation();
  const [confirm, setConfirm] = useState(false);
  const del = async () => {
    try {
      await api.deleteUser(user.id);
      onDeleted();
    } catch (e) {
      onError((e as Error).message);
    }
  };
  return (
    <Card size="sm" className="mb-4 border-destructive/70">
      <CardContent>
      <h2 className="m-0 mb-3 text-base font-semibold text-destructive">{t("common.danger-zone")}</h2>
      <p className="text-sm leading-relaxed text-muted-foreground">
        {t("user.delete-note")}
        {self && ` ${t("user.delete-self-note")}`}
      </p>
      <Button variant="destructive" type="button" disabled={self} onClick={() => setConfirm(true)}>{t("user.delete")}</Button>
      <ConfirmModal
        open={confirm}
        title={`Delete user "${user.username}"?`}
        message="This revokes all of the user's tokens and role assignments. This cannot be undone."
        confirmLabel={t("common.delete")}
        danger
        onConfirm={() => { setConfirm(false); del(); }}
        onCancel={() => setConfirm(false)}
      />
      </CardContent>
    </Card>
  );
}
