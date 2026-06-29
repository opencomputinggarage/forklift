import { screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { adminMe, mockApi, unauthenticatedMe } from "@/test/mock-api";
import { renderRoute } from "@/test/render-route";

const protectedRoutes = [
  "/",
  "/workspace",
  "/workspace/repositories",
  "/workspace/repositories/new",
  "/workspace/repositories/1",
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
  "/access",
  "/access/users",
  "/access/users/new",
  "/access/users/1",
  "/access/users/1/tokens/new",
  "/access/roles",
  "/access/roles/new",
  "/access/roles/1",
  "/admin",
  "/admin/ha",
  "/admin/notifications",
  "/admin/notifications/new",
  "/admin/notifications/1",
  "/system",
  "/system/settings",
] as const;

describe("application routes", () => {
  beforeEach(() => {
    window.localStorage.setItem("forklift.language", "en");
  });

  it("routes unauthenticated users to the login page", async () => {
    mockApi(unauthenticatedMe);
    const router = renderRoute("/workspace/repositories");

    await waitFor(() => expect(router.state.location.pathname).toBe("/login"));
    expect(await screen.findByRole("button", { name: "Sign in" })).toBeInTheDocument();
  });

  it("routes authenticated users away from the login page", async () => {
    mockApi(adminMe);
    const router = renderRoute("/login");

    await waitFor(() => expect(router.state.location.pathname).toBe("/workspace/repositories"));
  });

  it.each(protectedRoutes)("renders %s for an admin user", async (path) => {
    mockApi(adminMe);
    const router = renderRoute(path);

    expect((await screen.findAllByText("Repositories")).length).toBeGreaterThan(0);
    await waitFor(() => expect(router.state.status).toBe("idle"));
    expect(document.body).toHaveTextContent("forklift");
  });
});
