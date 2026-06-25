import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "@/authContext";
import { PageHeader } from "@/components/app-ui/page";
import { Receivers } from "../../notifications/index";

export const Route = createFileRoute("/admin/notifications/")({
  component: AdminNotificationsRoute,
});

function AdminNotificationsRoute() {
  const { me } = useAuth();
  if (!me.admin) return <Navigate to="/repositories" replace />;

  return (
    <>
      <PageHeader title="Notifications" />
      <Receivers />
    </>
  );
}
