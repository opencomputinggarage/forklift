import { useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate } from "@tanstack/react-router";
import { api, type Receiver } from "@/api";
import { useAuth } from "@/authContext";
import { ConfirmModal } from "@/components/overlays/confirm-modal";
import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { DataTable, type ColumnDef } from "@/components/app-ui/table";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/admin/notifications/")({
  component: AdminNotificationsRoute,
});

function AdminNotificationsRoute() {
  const { me } = useAuth();
  if (!me.admin) return <Navigate to="/workspace/repositories" replace />;

  return (
    <>
      <PageHeader title="Notifications" />
      <Receivers />
    </>
  );
}

// Receivers lists notification receivers — named alarm channels (webhooks) that
// repositories can select to be alerted when a package is quarantined pending
// approval. Add/Edit open a separate page; delete is confirmed inline. Sending a
// test posts a sample payload to the stored webhook. Rendered as the
// standalone Notifications admin surface (the route gates admin-only access).
export function Receivers() {
  const navigate = useNavigate();
  const [receivers, setReceivers] = useState<Receiver[] | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [testing, setTesting] = useState<number | null>(null);
  const [deleting, setDeleting] = useState<Receiver | null>(null);
  const columns: ColumnDef<Receiver>[] = [
    {
      header: "Name",
      cell: ({ row }) => <span className="whitespace-nowrap">{row.original.name}</span>,
    },
    {
      header: "Description",
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.description || "—"}</span>,
    },
    {
      header: "Webhook",
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.webhook_configured ? "configured" : "—"}</span>,
    },
    {
      header: "Created by",
      cell: ({ row }) => row.original.created_by || <span className="text-muted-foreground">—</span>,
    },
    {
      header: "Created",
      cell: ({ row }) => <span className="whitespace-nowrap text-muted-foreground">{row.original.created_at?.slice(0, 10)}</span>,
    },
    {
      header: "Enabled",
      cell: ({ row }) => row.original.enabled
        ? <Badge variant="success">enabled</Badge>
        : <Badge variant="outline">disabled</Badge>,
    },
    {
      id: "actions",
      cell: ({ row }) => {
        const receiver = row.original;
        return (
          <div className="flex min-w-0 items-center justify-end gap-2 max-sm:flex-wrap">
            <Button variant="outline" type="button" disabled={testing === receiver.id}
              onClick={() => sendTest(receiver)}>
              {testing === receiver.id ? "Sending…" : "Test"}
            </Button>
            <Button variant="outline" type="button"
              onClick={() => navigate({ to: "/admin/notifications/$id", params: { id: String(receiver.id) } })}>
              Edit
            </Button>
            <Button variant="destructive" type="button" onClick={() => setDeleting(receiver)}>
              Delete
            </Button>
          </div>
        );
      },
    },
  ];

  const load = () =>
    api.listReceivers().then(setReceivers).catch((e) => setError((e as Error).message));
  useEffect(() => { load(); }, []);

  const sendTest = async (r: Receiver) => {
    setError("");
    setNotice("");
    setTesting(r.id);
    try {
      await api.testReceiver(r.id);
      setNotice(`Test notification sent to "${r.name}".`);
    } catch (e) {
      setError(`Test to "${r.name}" failed. ${(e as Error).message}`);
    } finally {
      setTesting(null);
    }
  };

  const del = async (r: Receiver) => {
    setError("");
    try {
      await api.deleteReceiver(r.id);
      setDeleting(null);
      load();
    } catch (e) {
      setError((e as Error).message);
      setDeleting(null);
    }
  };

  return (
    <>
      <PageDescription>
        A receiver is a webhook (Slack/Mattermost-compatible) alerted when a package enters a
        repository's approval queue. Each repository chooses which receivers to notify in its Settings.
      </PageDescription>

      <div className="mb-4 flex min-w-0 items-center justify-between gap-3 max-sm:flex-col max-sm:items-start">
        <h2 className="m-0 text-base font-semibold">
          Receivers <span className="text-xs font-normal text-muted-foreground">· alarm channels</span>
        </h2>
        <Button onClick={() => navigate({ to: "/admin/notifications/new" })}>
          Add receiver
        </Button>
      </div>
      {error && <Alert className="mb-4">{error}</Alert>}
      {notice && <div className="mb-4 text-sm text-muted-foreground">{notice}</div>}
      {!receivers ? (
        <div className="text-sm text-muted-foreground">Loading…</div>
      ) : (
        <DataTable columns={columns} data={receivers} empty="No receivers yet. Add one to start sending approval alarms." />
      )}
      <p className="mt-4 text-sm text-muted-foreground">
        The webhook URL is never shown again after it is saved. Leave it blank when editing to keep the current URL.
      </p>

      <ConfirmModal
        open={deleting !== null}
        title={deleting ? `Delete receiver "${deleting.name}"?` : "Delete receiver"}
        message="Repositories selecting this receiver will stop notifying it. This cannot be undone."
        confirmLabel="Delete"
        danger
        onConfirm={() => deleting && del(deleting)}
        onCancel={() => setDeleting(null)}
      />
    </>
  );
}
