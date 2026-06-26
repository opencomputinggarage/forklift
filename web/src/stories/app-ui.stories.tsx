import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { PageDescription, PageHeader } from "@/components/app-ui/page";
import { Card, CardContent } from "@/components/ui/card";
import { Select } from "@/components/app-ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableWrap } from "@/components/app-ui/table";
import { StateBadge, ApprovalStatusBadge } from "@/components/app-ui/status-badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

const meta: Meta = {
  title: "Forklift/App UI",
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;

type Story = StoryObj;

export const Components: Story = {
  render: () => {
    const [repoType, setRepoType] = useState("proxy");
    const [enabled, setEnabled] = useState(true);

    return (
      <main className="min-h-screen bg-background p-8 text-foreground">
        <PageHeader
          title={
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <span>Repository policy</span>
              <Badge variant="secondary">{repoType}</Badge>
              <Badge>npm</Badge>
            </div>
          }
          actions={
            <>
              <Button variant="outline">Cancel</Button>
              <Button>Save changes</Button>
            </>
          }
        />
        <PageDescription>
          A compact sample of the app-specific wrappers built on top of shadcn/base-ui components.
        </PageDescription>

        <div className="grid gap-4 lg:grid-cols-[1fr_24rem]">
          <Card size="sm" className="mb-4">
            <CardContent>
              <h2 className="m-0 mb-4 text-base font-semibold">Controls</h2>
              <FieldGroup className="gap-4">
                <Field>
                  <FieldLabel>Repository type</FieldLabel>
                  <Select
                    value={repoType}
                    onChange={setRepoType}
                    options={[
                      { value: "proxy", label: "Proxy" },
                      { value: "hosted", label: "Hosted" },
                      { value: "group", label: "Group" },
                    ]}
                  />
                </Field>
                <Field>
                  <FieldLabel>Name</FieldLabel>
                  <Input defaultValue="npm-public" />
                </Field>
                <Field>
                  <FieldLabel>Policy note</FieldLabel>
                  <Textarea defaultValue="Review package approvals before serving new dependencies." />
                </Field>
                <label className="flex items-center gap-2 text-sm">
                  <Checkbox checked={enabled} onCheckedChange={(checked) => setEnabled(Boolean(checked))} />
                  <span>Enable policy</span>
                </label>
              </FieldGroup>
            </CardContent>
          </Card>

          <Card size="sm" className="mb-4">
            <CardContent>
              <h2 className="m-0 mb-4 text-base font-semibold">Badges</h2>
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <Badge>default</Badge>
                <Badge variant="success">success</Badge>
                <Badge variant="warning">warning</Badge>
                <Badge variant="destructive">destructive</Badge>
                <StateBadge state={enabled ? "online" : "offline"}>{enabled ? "online" : "offline"}</StateBadge>
                <ApprovalStatusBadge status="pending" />
                <Badge className="tabular-nums">12</Badge>
                <Badge>admin</Badge>
                <Badge className="font-mono">npm-*: read,write</Badge>
                <Badge>approve</Badge>
              </div>
            </CardContent>
          </Card>
        </div>

        <Card size="sm" className="mb-4">
          <CardContent>
            <h2 className="m-0 mb-4 text-base font-semibold">Table</h2>
            <TableWrap>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Repository</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Permissions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell className="font-mono text-xs">npm-public</TableCell>
                    <TableCell><Badge variant="secondary">{repoType}</Badge></TableCell>
                    <TableCell><StateBadge state={enabled ? "online" : "offline"}>{enabled ? "online" : "offline"}</StateBadge></TableCell>
                    <TableCell><Badge className="font-mono">npm-*: read,write</Badge></TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TableWrap>
          </CardContent>
        </Card>

        <Alert>Package approval scan failed. Review OSV connectivity.</Alert>
      </main>
    );
  },
};
