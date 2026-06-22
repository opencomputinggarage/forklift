import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../App";
import { RepositoryNew } from "../pages/RepositoryNew";

export const Route = createFileRoute("/repositories/new")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  return me.admin ? <RepositoryNew /> : <Navigate to="/repositories" replace />;
}
