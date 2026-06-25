import { createFileRoute, Navigate, useParams } from "@tanstack/react-router";

export const Route = createFileRoute("/admin/$tab")({
  component: AdminTabRedirect,
});

function AdminTabRedirect() {
  const { tab } = useParams({ from: "/admin/$tab" });
  return <Navigate to={tab === "notifications" ? "/admin/notifications" : "/admin/ha"} replace />;
}
