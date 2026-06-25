import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "@/authContext";
import { ReceiverForm } from "../../notifications/new";

export const Route = createFileRoute("/admin/notifications/new")({
  component: AdminReceiverNewRoute,
});

function AdminReceiverNewRoute() {
  const { me } = useAuth();
  return me.admin ? <ReceiverForm /> : <Navigate to="/repositories" replace />;
}
