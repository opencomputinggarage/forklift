import { createFileRoute, Navigate } from "@tanstack/react-router";

export const Route = createFileRoute("/workspace/")({
  component: () => <Navigate to="/workspace/repositories" replace />,
});
