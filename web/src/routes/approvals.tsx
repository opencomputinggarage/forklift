import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../App";
import { Approvals } from "../pages/Approvals";

export const Route = createFileRoute("/approvals")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  return me.admin || me.approver ? <Approvals /> : <Navigate to="/repositories" replace />;
}
