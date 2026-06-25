import { createFileRoute, Navigate } from "@tanstack/react-router";

export const Route = createFileRoute("/system/")({
  component: () => <Navigate to="/system/settings" replace />,
});
