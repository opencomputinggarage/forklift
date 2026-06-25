import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/workspace/approvals")({
  component: Outlet,
});
