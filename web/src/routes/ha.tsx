import { useEffect, useState } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { api, type HAStatus as HAStatusType } from "@/api";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Inline, PageDescription, PageHeader, Panel, PanelBody } from "@/components/app-ui/page";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
  TableWrap,
} from "@/components/app-ui/table";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/ha")({
  component: HAStatusRoute,
});

function HAStatusRoute() {
  const [status, setStatus] = useState<HAStatusType | null>(null);
  const [error, setError] = useState("");

  const load = () =>
    api
      .getHA()
      .then((nextStatus) => {
        setStatus(nextStatus);
        setError("");
      })
      .catch((err) => setError((err as Error).message));

  useEffect(() => {
    load();
    const timer = window.setInterval(load, 5_000);
    return () => window.clearInterval(timer);
  }, []);

  return (
    <>
      <PageHeader
        title="High Availability"
        actions={
          <Button variant="outline" type="button" onClick={load}>
            Refresh
          </Button>
        }
      />
      <PageDescription>
        Review storage topology, leader election state, and fencing token for the active cluster.
      </PageDescription>

      <Panel>
        <PanelBody>
          <Inline className="mb-4 justify-between gap-3 max-sm:flex-col max-sm:items-start">
            <h2 className="m-0 text-base font-semibold">Cluster status</h2>
            <span className="text-sm text-muted-foreground">auto-refresh · 5s</span>
          </Inline>

          {error && <Alert className="mb-4">{error}</Alert>}
          {!status ? (
            <p className="m-0 text-sm text-muted-foreground">Loading...</p>
          ) : (
            <TableWrap>
              <Table>
                <TableBody>
                  <TableRow><TableCell className="w-44 text-muted-foreground">Mode</TableCell><TableCell>{status.mode}</TableCell></TableRow>
                  <TableRow><TableCell className="text-muted-foreground">Storage backend</TableCell><TableCell>{status.backend}</TableCell></TableRow>
                  <TableRow><TableCell className="text-muted-foreground">Leader election</TableCell><TableCell>{status.enabled ? "enabled" : "disabled (single instance)"}</TableCell></TableRow>
                  <TableRow><TableCell className="text-muted-foreground">This pod</TableCell><TableCell className="font-mono text-xs">{status.identity || "-"}</TableCell></TableRow>
                  <TableRow>
                    <TableCell className="text-muted-foreground">Role</TableCell>
                    <TableCell>
                      <Inline className="gap-2">
                        <Badge variant={status.is_leader ? "success" : "outline"}>
                          {status.role || "-"}
                        </Badge>
                        {status.is_leader && <span className="text-sm text-muted-foreground">serving traffic</span>}
                      </Inline>
                    </TableCell>
                  </TableRow>
                  <TableRow><TableCell className="text-muted-foreground">Current leader</TableCell><TableCell className="font-mono text-xs">{status.leader || "-"}</TableCell></TableRow>
                  {status.lease_name && <TableRow><TableCell className="text-muted-foreground">Lease</TableCell><TableCell className="font-mono text-xs">{status.lease_name}</TableCell></TableRow>}
                  {typeof status.fencing_token === "number" && status.fencing_token > 0 && (
                    <TableRow><TableCell className="text-muted-foreground">Fencing token</TableCell><TableCell>{status.fencing_token}</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </TableWrap>
          )}

          <p className="mb-0 mt-4 text-sm leading-relaxed text-muted-foreground">
            In HA only the leader serves traffic; standby pods stay ready and take over automatically on failover.
            The fencing token guards object-storage metadata against a superseded leader.
          </p>
        </PanelBody>
      </Panel>
    </>
  );
}
