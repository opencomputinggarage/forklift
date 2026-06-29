import { vi } from "vitest";
import {
  api,
  type Approval,
  type ArtifactList,
  type AuditLogList,
  type HAStatus,
  type Me,
  type Receiver,
  type RepoPermission,
  type Repository,
  type RepositoryName,
  type RepoToken,
  type Role,
  type Token,
  type User,
  type Version,
  type VersionDenyList,
} from "@/api";

export const adminMe: Me = {
  authenticated: true,
  username: "admin",
  source: "local",
  admin: true,
  approver: true,
  auditor: true,
};

export const unauthenticatedMe: Me = { authenticated: false };

export const sampleRepository: Repository = {
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

export const sampleRole: Role = {
  id: 1,
  name: "administrator",
  description: "Full access",
  created_at: "2026-01-01T00:00:00Z",
  permissions: [
    {
      id: 1,
      repo_pattern: "*",
      actions: ["admin"],
    },
  ],
  user_count: 1,
  managed: false,
};

export const sampleUser: User = {
  id: 1,
  username: "admin",
  source: "local",
  email: "admin@example.com",
  disabled: false,
  created_at: "2026-01-01T00:00:00Z",
  last_login_at: "2026-01-02T00:00:00Z",
  roles: [{ id: sampleRole.id, name: sampleRole.name }],
  lockout_enabled: false,
  locked: false,
  protected: true,
};

const sampleToken: Token = {
  id: 1,
  name: "ci",
  description: "CI token",
  scopes_json: "[]",
  expires_at: null,
  last_used_at: null,
  created_at: "2026-01-01T00:00:00Z",
};

const sampleApproval: Approval = {
  id: 1,
  repo_name: sampleRepository.name,
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
  reviewers: ["admin"],
};

const sampleReceiver: Receiver = {
  id: 1,
  name: "slack",
  description: "Slack alerts",
  webhook_configured: true,
  enabled: true,
  created_by: "admin",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const sampleHA: HAStatus = {
  enabled: false,
  mode: "single",
  backend: "none",
  identity: "local",
  leader: "local",
  is_leader: true,
  role: "leader",
  version: "test",
};

const version: Version = {
  version: "test",
  commit: "none",
  oidc_enabled: false,
};

export function mockApi(me: Me = adminMe) {
  spy("me", me);
  spy("version", version);
  spy("login", { username: me.username ?? "admin" });
  spy("logout", undefined);
  spy("listRepositories", [sampleRepository]);
  spy("listRepositoryNames", [{ name: sampleRepository.name, format: sampleRepository.format, type: sampleRepository.type } satisfies RepositoryName]);
  spy("getRepository", sampleRepository);
  spy("createRepository", sampleRepository);
  spy("updateRepository", sampleRepository);
  spy("deleteRepository", undefined);
  spy("setRepositoryDisabled", sampleRepository);
  spy("checkUpstream", { applicable: true, reachable: true, status: 200, latency_ms: 12 });
  spy("purgeArtifacts", { deleted: 0 });
  spy("repositoryPermissions", [{ role_id: sampleRole.id, role: sampleRole.name, repo_pattern: "*", actions: ["admin"], user_count: 1 } satisfies RepoPermission]);
  spy("repositoryTokens", [{ token_id: sampleToken.id, name: sampleToken.name, owner: sampleUser.username, repo_pattern: "*", actions: ["read"], unscoped: false, expires_at: null, last_used_at: null } satisfies RepoToken]);
  spy("listArtifacts", { count: 0, total_size: 0, artifacts: [] } satisfies ArtifactList);
  spy("deleteArtifact", undefined);
  spy("listAuditLogs", { count: 0, logs: [] } satisfies AuditLogList);
  spy("listTokens", [sampleToken]);
  spy("createToken", { token: "plain-token", name: "ci" });
  spy("deleteToken", undefined);
  spy("createUserToken", { token: "plain-token", name: "ci" });
  spy("deleteUserToken", undefined);
  spy("listUsers", [sampleUser]);
  spy("createUser", { id: 2, username: "developer" });
  spy("updateUser", sampleUser);
  spy("deleteUser", undefined);
  spy("listUserTokens", [sampleToken]);
  spy("assignRole", undefined);
  spy("removeRole", undefined);
  spy("listRoles", [sampleRole]);
  spy("createRole", { ...sampleRole, id: 2, name: "developer", description: "Developer access" });
  spy("deleteRole", undefined);
  spy("addPermission", { id: 2, repo_pattern: "*", actions: ["read"] });
  spy("deletePermission", undefined);
  spy("listApprovals", { count: 1, approvals: [sampleApproval] });
  spy("approvalCount", { count: 1 });
  spy("getApproval", sampleApproval);
  spy("approveApproval", { ...sampleApproval, status: "approved", decided_by: "admin", decided_at: "2026-01-01T00:00:00Z" });
  spy("rejectApproval", { ...sampleApproval, status: "rejected", decided_by: "admin", decided_at: "2026-01-01T00:00:00Z" });
  spy("approveAllPending", { approved: 1 });
  spy("createApproval", sampleApproval);
  spy("listVersionDenies", { count: 0, denies: [] } satisfies VersionDenyList);
  spy("createVersionDeny", { id: 1, repo_name: sampleRepository.name, package: "left-pad", version: "1.0.0", reason: "test", created_by: "admin", created_at: "2026-01-01T00:00:00Z" });
  spy("deleteVersionDeny", undefined);
  spy("getHA", sampleHA);
  spy("stepDownHA", { status: "ok" });
  spy("listReceivers", [sampleReceiver]);
  spy("createReceiver", sampleReceiver);
  spy("updateReceiver", sampleReceiver);
  spy("deleteReceiver", undefined);
  spy("testReceiver", { status: "ok" });
  spy("testWebhookURL", { status: "ok" });
  spy("previewRepoSample", { payload: { repo: sampleRepository.name }, receivers: [{ name: sampleReceiver.name, exists: true, enabled: true }] });
  spy("sendRepoSample", { results: [{ name: sampleReceiver.name, ok: true }] });
}

function spy<TName extends keyof typeof api>(
  name: TName,
  value: Awaited<ReturnType<(typeof api)[TName]>>,
) {
  vi.spyOn(api, name).mockResolvedValue(value as never);
}
