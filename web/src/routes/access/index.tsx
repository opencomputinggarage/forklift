import { createFileRoute, Navigate } from "@tanstack/react-router";

export const Route = createFileRoute("/access/")({
  component: () => <Navigate to="/access/users" replace />,
});
