import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRouter, RouterProvider } from "@tanstack/react-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { api, type HAStatus, type Me, type Repository, type Role, type User, type Version } from "@/api";
import { routeTree } from "@/generated/route-tree.gen";

const repo: Repository = {
  id: 1,
  name: "npm-public",
  format: "npm",
  type: "proxy",
  upstream_url: "https://registry.npmjs.org",
  disabled: false,
  artifact_count: 1,
  total_size: 1024,
  pending_approval_count: 1,
  config: {
    cache: {
      enabled: true,
      artifact_ttl: "168h",
      metadata_ttl: "1h",
      negative_ttl: "5m",
      max_size_bytes: 1073741824,
      eviction: "lru",
    },
    age_policy: {
      enabled: true,
      min_age: "24h",
      max_age: "",
      action: "warn",
    },
    approval: {
      enabled: true,
      mode: "enforce",
      auto_approve: [],
    },
    retention: {
      idle_ttl: "720h",
    },
    vuln: {
      enabled: true,
      threshold: "high",
      action: "block",
      ignore: [],
      block_unscanned: false,
    },
    license: {
      enabled: true,
      action: "warn",
      deny: ["GPL-3.0"],
      allow: [],
      block_unresolved: false,
    },
    group: {
      members: [],
    },
    ip_acl: {
      enabled: false,
      allow: [],
    },
    notify: {
      receivers: ["slack"],
    },
  },
};

const role: Role = {
  id: 1,
  name: "administrator",
  description: "Full access",
  created_at: "2026-01-01T00:00:00Z",
  permissions: [{ id: 1, repo_pattern: "*", actions: ["admin"] }],
  user_count: 1,
  managed: false,
};

const me: Me = {
  authenticated: true,
  username: "admin",
  source: "local",
  admin: true,
  approver: true,
  auditor: true,
};

const user: User = {
  id: 1,
  username: "admin",
  source: "local",
  email: "admin@example.com",
  disabled: false,
  created_at: "2026-01-01T00:00:00Z",
  last_login_at: "2026-01-02T00:00:00Z",
  roles: [{ id: role.id, name: role.name }],
  lockout_enabled: false,
  locked: false,
  protected: true,
};

const version: Version = {
  version: "storybook",
  commit: "none",
  oidc_enabled: false,
};

const ha: HAStatus = {
  enabled: false,
  mode: "single",
  backend: "none",
  identity: "local",
  leader: "local",
  is_leader: true,
  role: "leader",
  version: "storybook",
};

const screenPaths = [
  "/workspace/repositories",
  "/workspace/repositories/new",
  "/workspace/repositories/1/artifacts",
  "/workspace/repositories/1/approvals",
  "/workspace/repositories/1/permissions",
  "/workspace/repositories/1/audit",
  "/workspace/repositories/1/security",
  "/workspace/repositories/1/settings",
  "/workspace/tokens",
  "/workspace/tokens/new",
  "/workspace/approvals",
  "/workspace/approvals/1",
  "/access/users",
  "/access/users/new",
  "/access/users/1",
  "/access/roles",
  "/access/roles/new",
  "/access/roles/1",
  "/admin/ha",
  "/admin/notifications",
  "/admin/notifications/new",
  "/admin/notifications/1",
  "/system/settings",
] as const;

const meta: Meta<{ path: (typeof screenPaths)[number] }> = {
  title: "Forklift/Screens",
  parameters: {
    layout: "fullscreen",
  },
  argTypes: {
    path: {
      control: "select",
      options: screenPaths,
    },
  },
  args: {
    path: "/workspace/repositories",
  },
};

export default meta;

type Story = StoryObj<typeof meta>;

export const AdminRoutes: Story = {
  render: ({ path }) => {
    installApiMocks();

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
          staleTime: 0,
        },
      },
    });
    const router = createRouter({
      routeTree,
      history: createMemoryHistory({ initialEntries: [path] }),
    });

    return (
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    );
  },
};

function installApiMocks() {
  const storyApi = api as unknown as Record<string, (...args: never[]) => Promise<unknown>>;
  storyApi.me = async () => me;
  storyApi.version = async () => version;
  storyApi.listRepositories = async () => [repo];
  storyApi.listRepositoryNames = async () => [{ name: repo.name, format: repo.format, type: repo.type }];
  storyApi.getRepository = async () => repo;
  storyApi.listArtifacts = async () => ({ count: 0, total_size: 0, artifacts: [] });
  storyApi.listAuditLogs = async () => ({ count: 0, logs: [] });
  storyApi.repositoryPermissions = async () => [{ role_id: role.id, role: role.name, repo_pattern: "*", actions: ["admin"], user_count: 1 }];
  storyApi.repositoryTokens = async () => [];
  storyApi.listTokens = async () => [];
  storyApi.listUsers = async () => [user];
  storyApi.listUserTokens = async () => [];
  storyApi.listRoles = async () => [role];
  storyApi.listApprovals = async () => ({ count: 0, approvals: [] });
  storyApi.approvalCount = async () => ({ count: 0 });
  storyApi.getApproval = async () => ({
    id: 1,
    repo_name: repo.name,
    package: "left-pad",
    status: "pending",
    requested_by: "developer",
    decided_by: "",
    note: "",
    request_count: 1,
    last_requested_version: "1.0.0",
    first_requested_at: "2026-01-01T00:00:00Z",
    last_requested_at: "2026-01-01T00:00:00Z",
    decided_at: null,
  });
  storyApi.listVersionDenies = async () => ({ count: 0, denies: [] });
  storyApi.getHA = async () => ha;
  storyApi.listReceivers = async () => [{
    id: 1,
    name: "slack",
    description: "Slack alerts",
    webhook_configured: true,
    enabled: true,
    created_by: "admin",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  }];
  storyApi.previewRepoSample = async () => ({ payload: { repo: repo.name }, receivers: [] });
  storyApi.checkUpstream = async () => ({ applicable: true, reachable: true, status: 200, latency_ms: 12 });

  globalThis.fetch = async (input: RequestInfo | URL) => {
    const url = String(input);
    let body: unknown = {};
    if (url.includes("/api/v1/repositories")) body = [repo];
    if (url.includes("/api/v1/approvals/count")) body = { count: 0 };
    if (url.includes("/api/v1/version")) body = version;
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
}
