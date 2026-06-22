import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../App";
import { RoleNew } from "../pages/RoleNew";

export const Route = createFileRoute("/roles/new")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  return me.admin ? <RoleNew /> : <Navigate to="/repositories" replace />;
}
