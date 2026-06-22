import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../App";
import { UserModify } from "../pages/UserModify";

export const Route = createFileRoute("/users/$id")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  const { id } = Route.useParams();
  return me.admin || me.auditor ? <UserModify me={me} id={id} /> : <Navigate to="/repositories" replace />;
}
