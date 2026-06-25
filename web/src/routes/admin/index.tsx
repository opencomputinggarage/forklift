import { createFileRoute, Navigate } from "@tanstack/react-router";

export const Route = createFileRoute("/admin/")({
  // The Admin page is tab-driven; land on the first tab.
  component: () => <Navigate to="/admin/$tab" params={{ tab: "ha" }} replace />,
});
