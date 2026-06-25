import { createFileRoute, Navigate, Outlet } from "@tanstack/react-router";
import { useAuth } from "@/authContext";

export const Route = createFileRoute("/admin/notifications")({
  component: AdminNotificationsLayout,
});

function AdminNotificationsLayout() {
  const { me } = useAuth();
  if (!me.admin) return <Navigate to="/repositories" replace />;
  return <Outlet />;
}
