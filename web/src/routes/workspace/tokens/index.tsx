import { useEffect, useState } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { api, Token } from "@/api";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type ColumnDef } from "@/components/app-ui/table";

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
  const navigate = useNavigate();
  const [tokens, setTokens] = useState<Token[]>([]);
  const [error, setError] = useState("");
  const [revokeId, setRevokeId] = useState<number | null>(null);
  const columns: ColumnDef<Token>[] = [
    {
      header: "Name",
      cell: ({ row }) => row.original.name,
    },
    {
      header: "Description",
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.description}</span>,
    },
    {
      header: "Permissions",
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
      header: "Created",
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.created_at?.slice(0, 10)}</span>,
    },
    {
      header: "Expires",
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.expires_at ? row.original.expires_at.slice(0, 10) : "never"}</span>,
    },
    {
      header: "Last used",
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.last_used_at ? row.original.last_used_at.slice(0, 10) : "never"}</span>,
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="text-right">
          <Button variant="destructive" onClick={() => setRevokeId(row.original.id)}>Revoke</Button>
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
        title="Personal access tokens"
        actions={
          <Button onClick={() => navigate({ to: "/workspace/tokens/new" })}>
            New token
          </Button>
        }
      />
      <PageDescription>
        Scoped credentials for package clients, limited to chosen repositories and actions
        within your own permissions. Use a token as the password in your package manager
        (npm <code>_authToken</code>, Maven, Cargo, <code>.netrc</code> for Go).
      </PageDescription>

      {error && <Alert className="mb-4">{error}</Alert>}

      <DataTable columns={columns} data={tokens} empty="No tokens yet." />

      <ConfirmModal
        open={revokeId !== null}
        title="Revoke this token?"
        message="Clients using this token will immediately lose access. This cannot be undone."
        confirmLabel="Revoke"
        danger
        onConfirm={revoke}
        onCancel={() => setRevokeId(null)}
      />
    </>
  );
}
