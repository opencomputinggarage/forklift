import { createFileRoute, Navigate } from "@tanstack/react-router";

export const Route = createFileRoute("/admin/")({
  // Admin surfaces now live as first-class sidebar destinations.
  component: () => <Navigate to="/admin/ha" replace />,
});
