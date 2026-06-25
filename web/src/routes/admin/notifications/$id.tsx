import { createFileRoute, Navigate, useParams } from "@tanstack/react-router";
import { useAuth } from "@/authContext";
import { ReceiverForm } from "./new";

export const Route = createFileRoute("/admin/notifications/$id")({
  component: AdminReceiverEditRoute,
});

function AdminReceiverEditRoute() {
  const { me } = useAuth();
  const { id } = useParams({ from: "/admin/notifications/$id" });
  if (!me.admin) return <Navigate to="/workspace/repositories" replace />;
  return <ReceiverForm receiverId={Number(id)} />;
}
