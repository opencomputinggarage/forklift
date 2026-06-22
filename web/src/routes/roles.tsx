import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../App";
import { Roles } from "../pages/Roles";

export const Route = createFileRoute("/roles")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  return me.admin || me.auditor ? <Roles me={me} /> : <Navigate to="/repositories" replace />;
}
