import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../App";
import { ApprovalDetail } from "../pages/ApprovalDetail";

export const Route = createFileRoute("/approvals/$id")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  const { id } = Route.useParams();
  return me.admin || me.approver ? <ApprovalDetail id={id} /> : <Navigate to="/repositories" replace />;
}
