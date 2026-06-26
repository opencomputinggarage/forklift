import { useEffect, useState } from "react";
import { createFileRoute, Link, Navigate, useNavigate, useParams } from "@tanstack/react-router";
import { LockKeyhole, X } from "lucide-react";
import { api, Me, Role, User } from "@/api";
import { useAuth } from "@/authContext";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from "@/components/ui/combobox";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { Alert } from "@/components/app-ui/alert";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/app-ui/badge";
import { StateBadge } from "@/components/app-ui/status-badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrap,
} from "@/components/app-ui/table";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/access/roles/$id")({
  component: RoleModifyRoute,
});

function RoleModifyRoute() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <RoleModify me={me} /> : <Navigate to="/workspace/repositories" replace />;
}

const ACTIONS = ["read", "write", "delete", "approve", "audit", "admin"];

// Per-role modify page: permission mapping, assigned users, and the danger zone
// (delete). The Roles list is read-only; all edits happen here. The page is
// read-only (no add/remove permission, no delete) for an auditor and for managed
// roles, which are owned by the chart's declarative RBAC policy.
export function RoleModify({ me }: { me: Me }) {
  const { t } = useTranslation();
  const { id } = useParams({ strict: false }) as { id?: string };
  const navigate = useNavigate();
  const roleId = Number(id);
  const [role, setRole] = useState<Role | null>(null);
  const [members, setMembers] = useState<User[]>([]);
  const [error, setError] = useState("");

  const load = () =>
    Promise.all([api.listRoles(), api.listUsers()])
      .then(([roles, users]) => {
        const r = roles.find((x) => x.id === roleId) ?? null;
        setRole(r);
        setMembers(users.filter((u) => u.roles.some((ur) => ur.id === roleId)));
        if (!r) setError("Role not found.");
      })
      .catch((e) => setError(e.message));
  useEffect(() => { load(); /* eslint-disable-next-line */ }, [roleId]);

  if (error && !role) return <Alert className="my-2.5">{error}</Alert>;
  if (!role) return <div className="text-sm text-muted-foreground">{t("common.loading")}</div>;

  const run = (p: Promise<unknown>) => {
    setError("");
    p.then(load).catch((e) => setError((e as Error).message));
  };

  // Managed roles are reconciled from the chart's declarative RBAC policy and are
  // read-only via the API. Gate every edit control on !role.managed so an admin
  // never sees a button that would only return a 409; the backend still enforces
  // this regardless of the UI.
  const editable = !!me.admin && !role.managed;

  return (
    <>
      <PageHeader
        title={role.name}
        actions={
        <Button variant="outline" onClick={() => navigate({ to: "/access/roles" })}>
          {t("role.back")}
        </Button>
        }
      />
      {role.description && <PageDescription>{role.description}</PageDescription>}
      {role.managed && (
        <Card size="sm" className="mb-4 border-primary/70">
          <CardContent>
          <h2 className="mb-2 flex items-center gap-2 text-base font-semibold">
            <LockKeyhole className="size-4 text-primary" aria-hidden="true" />
            {t("role.managed")}
          </h2>
          <p className="m-0 text-sm leading-relaxed text-muted-foreground">
            {t("role.managed-note")}
          </p>
          </CardContent>
        </Card>
      )}
      {error && <Alert className="mb-4">{error}</Alert>}

      <PermissionsPanel role={role} run={run} canWrite={editable} />
      <AssignedUsersPanel members={members} />
      {editable && <DangerPanel role={role} onDeleted={() => navigate({ to: "/access/roles" })} onError={setError} />}
    </>
  );
}

// AssignedUsersPanel lists the users that currently hold this role. Assignment
// itself is managed on each user's detail page, so this is read-only with links.
function AssignedUsersPanel({ members }: { members: User[] }) {
  const { t } = useTranslation();
  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">
        {t("role.assigned-users")} <Badge className="ml-1.5 tabular-nums">{members.length}</Badge>
      </h2>
      {members.length === 0
        ? <p className="m-0 text-sm text-muted-foreground">{t("role.no-users")}</p>
        : (
          // Same column structure and order as the Users page; the username
          // links to that user's detail page.
          <TableWrap>
          <Table>
            <TableHeader>
              <TableRow><TableHead>{t("common.username")}</TableHead><TableHead>{t("common.source")}</TableHead><TableHead>{t("common.email")}</TableHead><TableHead>{t("common.roles")}</TableHead><TableHead>{t("common.status")}</TableHead><TableHead>{t("common.last-login")}</TableHead></TableRow>
            </TableHeader>
            <TableBody>
              {members.map((u) => (
                <TableRow key={u.id}>
                  <TableCell className="whitespace-nowrap"><Link to="/access/users/$id" params={{ id: String(u.id) }}>{u.username}</Link></TableCell>
                  <TableCell><Badge>{u.source}</Badge></TableCell>
                  <TableCell className="text-muted-foreground">{u.email || "-"}</TableCell>
                  <TableCell>
                    <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                      {u.roles.map((r) => (
                        <Badge
                          key={r.id}
                          render={<Link to="/access/roles/$id" params={{ id: String(r.id) }} />}
                        >
                          {r.name}
                        </Badge>
                      ))}
                      {u.roles.length === 0 && <span className="text-muted-foreground">{t("common.none")}</span>}
                    </div>
                  </TableCell>
                  <TableCell>
                    {u.disabled
                      ? <StateBadge state="disabled">{t("common.status.disabled")}</StateBadge>
                      : <StateBadge state="active">{t("common.status.active")}</StateBadge>}
                  </TableCell>
                  <TableCell className="whitespace-nowrap text-muted-foreground" title={u.last_login_at ?? undefined}>
                    {u.last_login_at ? new Date(u.last_login_at).toLocaleString() : t("common.never")}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          </TableWrap>
        )}
      </CardContent>
    </Card>
  );
}

function PermissionsPanel({ role, run, canWrite }: { role: Role; run: (p: Promise<unknown>) => void; canWrite: boolean }) {
  const { t } = useTranslation();
  const [pattern, setPattern] = useState("");
  const [actions, setActions] = useState<string[]>(["read"]);
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

  const add = () => {
    run(api.addPermission(role.id, { repo_pattern: pattern.trim(), actions }));
    setPattern("");
    setActions(["read"]);
  };

  return (
    <Card size="sm" className="mb-4">
      <CardContent>
      <h2 className="m-0 mb-4 text-base font-semibold">{t("common.permissions")}</h2>
      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
        {role.permissions.map((p) => (
          <Badge key={p.id} className="gap-1 font-mono">
            <span>{p.repo_pattern}: {p.actions.join(",")}</span>
            {canWrite && (
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                className="-mr-1 size-4 rounded-full text-muted-foreground hover:bg-background/40 hover:text-foreground"
                title={t("common.remove-permission")}
                onClick={() => run(api.deletePermission(role.id, p.id))}
              >
                <X className="size-3" aria-hidden="true" />
                <span className="sr-only">{t("common.remove-permission")}</span>
              </Button>
            )}
          </Badge>
        ))}
        {role.permissions.length === 0 && <span className="text-sm text-muted-foreground">{t("common.no-permissions-granted")}</span>}
      </div>
      {canWrite && (
        <div className="flex min-w-0 items-center gap-2 mt-4 max-sm:flex-wrap flex-wrap items-stretch gap-2 max-sm:flex-col">
          <Combobox
            items={repoOptions}
            inputValue={pattern}
            value={repoOptions.includes(pattern) ? pattern : null}
            onInputValueChange={setPattern}
            onValueChange={(next) => {
              if (typeof next === "string") setPattern(next);
            }}
          >
            <ComboboxInput placeholder={t("common.repo-pattern-placeholder")} className="w-full sm:w-[200px]" />
            <ComboboxContent>
              <ComboboxEmpty>{t("common.no-repositories-found")}</ComboboxEmpty>
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
          {ACTIONS.map((a) => (
            <label key={a} className="flex items-center gap-2 text-xs">
              <Checkbox checked={actions.includes(a)} onCheckedChange={() => toggle(a)} />
              <span>{a}</span>
            </label>
          ))}
          <Button variant="outline" type="button"
            disabled={!pattern.trim() || actions.length === 0} onClick={add}>{t("common.add")}</Button>
        </div>
      )}
      </CardContent>
    </Card>
  );
}

function DangerPanel({ role, onDeleted, onError }: {
  role: Role; onDeleted: () => void; onError: (e: string) => void;
}) {
  const { t } = useTranslation();
  const [confirm, setConfirm] = useState(false);
  const del = async () => {
    try {
      await api.deleteRole(role.id);
      onDeleted();
    } catch (e) {
      onError((e as Error).message);
    }
  };
  return (
    <Card size="sm" className="mb-4 border-destructive/70">
      <CardContent>
      <h2 className="m-0 mb-3 text-base font-semibold text-destructive">{t("common.danger-zone")}</h2>
      <p className="text-sm leading-relaxed text-muted-foreground">{t("role.delete-confirm")}</p>
      <Button variant="destructive" type="button" onClick={() => setConfirm(true)}>{t("role.delete")}</Button>
      <ConfirmModal
        open={confirm}
        title={`Delete role "${role.name}"?`}
        message="Users and group mappings holding this role lose its permissions immediately."
        confirmLabel={t("common.delete")}
        danger
        onConfirm={() => { setConfirm(false); del(); }}
        onCancel={() => setConfirm(false)}
      />
      </CardContent>
    </Card>
  );
}
