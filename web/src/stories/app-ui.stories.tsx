import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { Alert } from "@/components/app-ui/alert";
import { Badge } from "@/components/app-ui/badge";
import { Inline, PageDescription, PageHeader, Panel, PanelBody } from "@/components/app-ui/page";
import { Select } from "@/components/app-ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow, TableWrap } from "@/components/app-ui/table";
import { ActionBadge, PermissionBadge, RoleBadge } from "@/components/app-ui/action-badge";
import { RepoTypeBadge, FormatBadge } from "@/components/app-ui/repo-type-badge";
import { StateBadge, ApprovalStatusBadge, CountBadge } from "@/components/app-ui/status-badge";
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
            <Inline className="flex-wrap gap-2">
              <span>Repository policy</span>
              <RepoTypeBadge type={repoType} />
              <FormatBadge format="npm" />
            </Inline>
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
          <Panel>
            <PanelBody>
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
            </PanelBody>
          </Panel>

          <Panel>
            <PanelBody>
              <h2 className="m-0 mb-4 text-base font-semibold">Badges</h2>
              <Inline className="flex-wrap gap-2">
                <Badge>default</Badge>
                <Badge variant="success">success</Badge>
                <Badge variant="warning">warning</Badge>
                <Badge variant="destructive">destructive</Badge>
                <StateBadge state={enabled ? "online" : "offline"}>{enabled ? "online" : "offline"}</StateBadge>
                <ApprovalStatusBadge status="pending" />
                <CountBadge>12</CountBadge>
                <RoleBadge>admin</RoleBadge>
                <PermissionBadge>npm-*: read,write</PermissionBadge>
                <ActionBadge action="approve" />
              </Inline>
            </PanelBody>
          </Panel>
        </div>

        <Panel>
          <PanelBody>
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
                    <TableCell><RepoTypeBadge type={repoType} /></TableCell>
                    <TableCell><StateBadge state={enabled ? "online" : "offline"}>{enabled ? "online" : "offline"}</StateBadge></TableCell>
                    <TableCell><PermissionBadge>npm-*: read,write</PermissionBadge></TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </TableWrap>
          </PanelBody>
        </Panel>

        <Alert>Package approval scan failed. Review OSV connectivity.</Alert>
      </main>
    );
  },
};
