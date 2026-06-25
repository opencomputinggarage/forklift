import { useEffect, useState } from "react";
import { createFileRoute, Navigate, useNavigate } from "@tanstack/react-router";
import { api } from "@/api";
import { useAuth } from "@/authContext";
import { Alert } from "@/components/app-ui/alert";
import { LockNote } from "@/components/app-ui/lock-note";
import { Inline, PageHeader, Panel, PanelBody } from "@/components/app-ui/page";
import { Toggle } from "@/components/inputs/toggle";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

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
      setTestErr("Enter a webhook URL first.");
      return;
    }
    setTesting(true);
    try {
      if (url) await api.testWebhookURL(url, form.name);
      else await api.testReceiver(receiverId!);
      setTestMsg("Test notification sent.");
    } catch (e) {
      setTestErr((e as Error).message);
    } finally {
      setTesting(false);
    }
  };

  if (!loaded) return <div className="text-sm text-muted-foreground">Loading…</div>;

  return (
    <>
      <PageHeader title={editing ? "Edit receiver" : "Add receiver"} />

      <Panel className="max-w-[44rem]">
        <PanelBody>
          <FieldGroup className="gap-4">
            <div className="grid gap-4 md:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="rcv-name">Name</FieldLabel>
                <Input id="rcv-name" value={form.name} placeholder="slack-security" autoFocus
                  onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </Field>
              <Field>
                <FieldLabel htmlFor="rcv-desc">Description</FieldLabel>
                <Input id="rcv-desc" value={form.description} placeholder="Security team Slack channel"
                  onChange={(e) => setForm({ ...form, description: e.target.value })} />
              </Field>
            </div>

            <Field>
              <FieldLabel htmlFor="rcv-url">
                Webhook URL
                {editing && <span className="ml-1 text-xs font-normal text-muted-foreground">· leave blank to keep current</span>}
              </FieldLabel>
              <Input id="rcv-url" value={form.webhook_url}
                placeholder={editing ? "•••••• (unchanged)" : "https://hooks.slack.com/services/…"}
                onChange={(e) => setForm({ ...form, webhook_url: e.target.value })} />
              <FieldDescription>Slack/Mattermost-compatible incoming webhook.</FieldDescription>
            </Field>

            <Inline className="gap-3">
              <Button variant="outline" type="button" disabled={testing} onClick={sendTest}>
                {testing ? "Sending…" : "Send test"}
              </Button>
              {testMsg && <span className="text-sm text-muted-foreground">{testMsg}</span>}
              {testErr && <span className="text-sm text-destructive">{testErr}</span>}
            </Inline>

            <Toggle checked={form.enabled} label={form.enabled ? "Enabled" : "Disabled"}
              onChange={(v) => setForm({ ...form, enabled: v })} />
          </FieldGroup>

          {error && <Alert className="mt-4">{error}</Alert>}

          <LockNote title="Webhook URL is write-only" className="mt-5">
            The webhook URL cannot be viewed again after you save it. You can only replace it with a new one.
            Keep a copy somewhere safe before you save.
          </LockNote>

          <Inline className="mt-5">
            <Button type="button" disabled={!form.name.trim()} onClick={save}>
              {editing ? "Save changes" : "Add receiver"}
            </Button>
            <Button variant="outline" type="button" onClick={() => navigate({ to: "/admin/notifications" })}>
              Cancel
            </Button>
          </Inline>
        </PanelBody>
      </Panel>
    </>
  );
}
