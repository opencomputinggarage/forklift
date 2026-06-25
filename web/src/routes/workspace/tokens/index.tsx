import { useEffect, useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { api, Token } from "@/api";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { PageDescription, PageHeader, Panel, PanelBody } from "@/components/app-ui/page";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrap,
} from "@/components/app-ui/table";

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
  const [tokens, setTokens] = useState<Token[]>([]);
  const [error, setError] = useState("");
  const [revokeId, setRevokeId] = useState<number | null>(null);

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
          <Button render={<Link to="/workspace/tokens/new" />} nativeButton={false}>
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

      <Panel>
        <PanelBody>
          <TableWrap>
          <Table>
            <TableHeader><TableRow><TableHead>Name</TableHead><TableHead>Description</TableHead><TableHead>Permissions</TableHead><TableHead>Created</TableHead><TableHead>Expires</TableHead><TableHead>Last used</TableHead><TableHead /></TableRow></TableHeader>
            <TableBody>
            {tokens.map((t) => (
              <TableRow key={t.id}>
                <TableCell>{t.name}</TableCell>
                <TableCell className="text-muted-foreground">{t.description}</TableCell>
                <TableCell>
                  {parseScopes(t.scopes_json).map((s, i) => (
                    <Badge key={i} className="mr-1 font-mono">
                      {s.repo_pattern}: {s.actions.join(",")}
                    </Badge>
                  ))}
                </TableCell>
                <TableCell className="text-muted-foreground">{t.created_at?.slice(0, 10)}</TableCell>
                <TableCell className="text-muted-foreground">{t.expires_at ? t.expires_at.slice(0, 10) : "never"}</TableCell>
                <TableCell className="text-muted-foreground">{t.last_used_at ? t.last_used_at.slice(0, 10) : "never"}</TableCell>
                <TableCell><Button variant="destructive" onClick={() => setRevokeId(t.id)}>Revoke</Button></TableCell>
              </TableRow>
            ))}
            {tokens.length === 0 && <TableRow><TableCell colSpan={7} className="text-muted-foreground">No tokens yet.</TableCell></TableRow>}
            </TableBody>
          </Table>
          </TableWrap>
        </PanelBody>
      </Panel>

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
