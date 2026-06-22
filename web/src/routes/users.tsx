import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../App";
import { Users } from "../pages/Users";

export const Route = createFileRoute("/users")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <Users me={me} /> : <Navigate to="/repositories" replace />;
}
