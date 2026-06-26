import { useEffect, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { api, Token } from "@/api";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type ColumnDef } from "@/components/app-ui/table";
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/workspace/tokens/")({
  component: Tokens,
});

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

export function Tokens() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [tokens, setTokens] = useState<Token[]>([]);
  const [error, setError] = useState("");
  const [revokeId, setRevokeId] = useState<number | null>(null);
  const columns: ColumnDef<Token>[] = [
    {
      header: t("common.name"),
      cell: ({ row }) => row.original.name,
    },
    {
      header: t("common.description"),
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.description}</span>,
    },
    {
      header: t("common.permissions"),
      cell: ({ row }) => (
        <>
          {parseScopes(row.original.scopes_json).map((scope, i) => (
            <Badge key={i} className="mr-1 font-mono">
              {scope.repo_pattern}: {scope.actions.join(",")}
            </Badge>
          ))}
        </>
      ),
    },
    {
      header: t("common.created"),
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.created_at?.slice(0, 10)}</span>,
    },
    {
      header: t("common.expires"),
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.expires_at ? row.original.expires_at.slice(0, 10) : t("common.never")}</span>,
    },
    {
      header: t("common.last-used"),
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.last_used_at ? row.original.last_used_at.slice(0, 10) : t("common.never")}</span>,
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="text-right">
          <Button variant="destructive" onClick={() => setRevokeId(row.original.id)}>{t("token.revoke")}</Button>
        </div>
      ),
    },
  ];

  const load = () => api.listTokens().then(setTokens).catch((e) => setError(e.message));
  useEffect(() => { load(); }, []);

  const revoke = async () => {
    if (revokeId === null) return;
    await api.deleteToken(revokeId);
    setRevokeId(null);
    load();
  };

  return (
    <>
      <PageHeader
        title={t("token.personal-title")}
        actions={
          <Button onClick={() => navigate({ to: "/workspace/tokens/new" })}>
            {t("token.new")}
          </Button>
        }
      />
      <PageDescription>
        {t("token.help-1")} <code>_authToken</code>, Maven, Cargo, <code>.netrc</code> {t("token.help-2")}
      </PageDescription>

      {error && <Alert className="mb-4">{error}</Alert>}

      <DataTable columns={columns} data={tokens} empty={t("token.empty")} />

      <ConfirmModal
        open={revokeId !== null}
        title={t("token.revoke-confirm-title")}
        message={t("token.revoke-confirm-message")}
        confirmLabel={t("token.revoke")}
        danger
        onConfirm={revoke}
        onCancel={() => setRevokeId(null)}
      />
    </>
  );
}
