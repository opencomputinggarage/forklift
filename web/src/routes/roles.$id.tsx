import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../App";
import { RoleModify } from "../pages/RoleModify";

export const Route = createFileRoute("/roles/$id")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  const { id } = Route.useParams();
  return me.admin || me.auditor ? <RoleModify me={me} id={id} /> : <Navigate to="/repositories" replace />;
}
