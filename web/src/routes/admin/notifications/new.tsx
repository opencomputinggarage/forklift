import { useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate } from "@tanstack/react-router";
import { LockKeyhole } from "lucide-react";
import { api } from "@/api";
import { useAuth } from "@/authContext";
import { Alert } from "@/components/app-ui/alert";
import { PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/admin/notifications/new")({
  component: AdminReceiverNewRoute,
});

function AdminReceiverNewRoute() {
  const { me } = useAuth();
  return me.admin ? <ReceiverForm /> : <Navigate to="/workspace/repositories" replace />;
}

// ReceiverForm is the shared create/edit form for a notification receiver,
// reached from the Receivers list. The webhook URL is write-only: it is never
// returned, so the field starts blank — on edit, leaving it blank keeps the
// stored URL; a non-empty value replaces it. Reused by the /admin/notifications/$id
// edit route by passing receiverId.
export function ReceiverForm({ receiverId }: { receiverId?: number }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const editing = receiverId !== undefined;
  const [form, setForm] = useState({ name: "", description: "", webhook_url: "", enabled: true });
  const [error, setError] = useState("");
  const [loaded, setLoaded] = useState(!editing);
  const [testing, setTesting] = useState(false);
  const [testMsg, setTestMsg] = useState("");
  const [testErr, setTestErr] = useState("");

  useEffect(() => {
    if (!editing) return;
    api.listReceivers()
      .then((rs) => {
        const r = rs.find((x) => x.id === receiverId);
        if (r) setForm({ name: r.name, description: r.description, webhook_url: "", enabled: r.enabled });
        setLoaded(true);
      })
      .catch((e) => { setError((e as Error).message); setLoaded(true); });
  }, [receiverId, editing]);

  const save = async () => {
    setError("");
    try {
      if (editing) await api.updateReceiver(receiverId, form);
      else await api.createReceiver(form);
      navigate({ to: "/admin/notifications" });
    } catch (e) {
      setError((e as Error).message);
    }
  };

  // Send a test alarm. A typed URL is tested ad-hoc (verify before saving); on
  // edit with the field left blank, the stored URL is tested instead.
  const sendTest = async () => {
    setTestMsg("");
    setTestErr("");
    const url = form.webhook_url.trim();
    if (!url && !editing) {
      setTestErr(t("notification.webhook-required"));
      return;
    }
    setTesting(true);
    try {
      if (url) await api.testWebhookURL(url, form.name);
      else await api.testReceiver(receiverId!);
      setTestMsg(t("notification.test-sent"));
    } catch (e) {
      setTestErr((e as Error).message);
    } finally {
      setTesting(false);
    }
  };

  if (!loaded) return <div className="text-sm text-muted-foreground">{t("common.loading")}</div>;

  return (
    <>
      <PageHeader title={editing ? t("notification.edit") : t("notification.add")} />

      <Card size="sm" className="mb-4 max-w-[44rem]">
        <CardContent>
          <FieldGroup className="gap-4">
            <div className="grid gap-4 md:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="rcv-name">{t("common.name")}</FieldLabel>
                <Input id="rcv-name" value={form.name} placeholder="slack-security" autoFocus
                  onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="rcv-desc">{t("common.description")}</FieldLabel>
                <Input id="rcv-desc" value={form.description} placeholder={t("notification.description-placeholder")}
                  onChange={(e) => setForm({ ...form, description: e.target.value })} />
              </Field>
            </div>

            <Field>
              <FieldLabel htmlFor="rcv-url">
                {t("common.webhook-url")}
                {editing && <span className="ml-1 text-xs font-normal text-muted-foreground">{t("notification.webhook-subtitle")}</span>}
              </FieldLabel>
              <Input id="rcv-url" value={form.webhook_url}
                placeholder={editing ? "•••••• (unchanged)" : "https://hooks.slack.com/services/…"}
                onChange={(e) => setForm({ ...form, webhook_url: e.target.value })} />
              <FieldDescription>{t("notification.webhook-hint")}</FieldDescription>
            </Field>

            <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap gap-3">
              <Button variant="outline" type="button" disabled={testing} onClick={sendTest}>
                {testing ? t("common.sending") : t("common.send-test")}
              </Button>
              {testMsg && <span className="text-sm text-muted-foreground">{testMsg}</span>}
              {testErr && <span className="text-sm text-destructive">{testErr}</span>}
            </div>

            <label className="inline-flex items-center gap-2 text-sm">
              <Switch
                checked={form.enabled}
                onCheckedChange={(v) => setForm({ ...form, enabled: v })}
                aria-label={form.enabled ? t("common.enabled") : t("common.disabled")}
              />
              <span>{form.enabled ? t("common.enabled") : t("common.disabled")}</span>
            </label>
          </FieldGroup>

          {error && <Alert className="mt-4">{error}</Alert>}

          <Card size="sm" className="mt-5 border-primary/70">
            <CardContent>
              <h2 className="mb-2 flex items-center gap-2 text-base font-semibold">
                <LockKeyhole className="size-4 text-primary" aria-hidden="true" />
                {t("notification.webhook-write-only")}
              </h2>
              <p className="m-0 text-sm leading-relaxed text-muted-foreground">
                {t("notification.webhook-warning")}
              </p>
            </CardContent>
          </Card>

          <div className="flex min-w-0 items-center gap-2 max-sm:flex-wrap mt-5">
            <Button type="button" disabled={!form.name.trim()} onClick={save}>
              {editing ? t("common.save-changes") : t("notification.add")}
            </Button>
            <Button variant="outline" type="button" onClick={() => navigate({ to: "/admin/notifications" })}>
              {t("common.cancel")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </>
  );
}
