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
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/admin/notifications/")({
  component: AdminNotificationsRoute,
});

function AdminNotificationsRoute() {
  const { me } = useAuth();
  const { t } = useTranslation();
  if (!me.admin) return <Navigate to="/workspace/repositories" replace />;

  return (
    <>
      <PageHeader title={t("notification.title")} />
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
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [receivers, setReceivers] = useState<Receiver[] | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [testing, setTesting] = useState<number | null>(null);
  const [deleting, setDeleting] = useState<Receiver | null>(null);
  const columns: ColumnDef<Receiver>[] = [
    {
      header: t("common.name"),
      cell: ({ row }) => <span className="whitespace-nowrap">{row.original.name}</span>,
    },
    {
      header: t("common.description"),
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.description || "—"}</span>,
    },
    {
      header: t("common.webhook"),
      cell: ({ row }) => <span className="text-muted-foreground">{row.original.webhook_configured ? t("common.status.configured") : "—"}</span>,
    },
    {
      header: t("common.created-by"),
      cell: ({ row }) => row.original.created_by || <span className="text-muted-foreground">—</span>,
    },
    {
      header: t("common.created"),
      cell: ({ row }) => <span className="whitespace-nowrap text-muted-foreground">{row.original.created_at?.slice(0, 10)}</span>,
    },
    {
      header: t("common.enabled"),
      cell: ({ row }) => row.original.enabled
        ? <Badge variant="success">{t("common.status.enabled")}</Badge>
        : <Badge variant="outline">{t("common.status.disabled")}</Badge>,
    },
    {
      id: "actions",
      cell: ({ row }) => {
        const receiver = row.original;
        return (
          <div className="flex min-w-0 items-center justify-end gap-2 max-sm:flex-wrap">
            <Button variant="outline" type="button" disabled={testing === receiver.id}
              onClick={() => sendTest(receiver)}>
              {testing === receiver.id ? t("common.sending") : t("common.test")}
            </Button>
            <Button variant="outline" type="button"
              onClick={() => navigate({ to: "/admin/notifications/$id", params: { id: String(receiver.id) } })}>
              {t("common.edit")}
            </Button>
            <Button variant="destructive" type="button" onClick={() => setDeleting(receiver)}>
              {t("common.delete")}
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
        {t("notification.description")}
      </PageDescription>

      <div className="mb-4 flex min-w-0 items-center justify-between gap-3 max-sm:flex-col max-sm:items-start">
        <h2 className="m-0 text-base font-semibold">
          {t("notification.receivers")} <span className="text-xs font-normal text-muted-foreground">{t("notification.subtitle")}</span>
        </h2>
        <Button onClick={() => navigate({ to: "/admin/notifications/new" })}>
          {t("notification.add")}
        </Button>
      </div>
      {error && <Alert className="mb-4">{error}</Alert>}
      {notice && <div className="mb-4 text-sm text-muted-foreground">{notice}</div>}
      {!receivers ? (
        <div className="text-sm text-muted-foreground">{t("common.loading")}</div>
      ) : (
        <DataTable columns={columns} data={receivers} empty={t("notification.empty")} />
      )}
      <p className="mt-4 text-sm text-muted-foreground">
        {t("notification.webhook-note")}
      </p>

      <ConfirmModal
        open={deleting !== null}
        title={deleting ? `Delete receiver "${deleting.name}"?` : t("notification.delete")}
        message={t("notification.delete-confirm")}
        confirmLabel={t("common.delete")}
        danger
        onConfirm={() => deleting && del(deleting)}
        onCancel={() => setDeleting(null)}
      />
    </>
  );
}
