import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../App";
import { UserNew } from "../pages/UserNew";

export const Route = createFileRoute("/users/new")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  return me.admin ? <UserNew /> : <Navigate to="/repositories" replace />;
}
