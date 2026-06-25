import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "@/authContext";
import { PageHeader } from "@/components/app-ui/page";
import { HAStatusPanel } from "../ha";

export const Route = createFileRoute("/admin/ha")({
  component: AdminHARoute,
});

function AdminHARoute() {
  const { me } = useAuth();
  if (!me.admin) return <Navigate to="/repositories" replace />;

  return (
    <>
      <PageHeader title="HA Status" />
      <HAStatusPanel />
    </>
  );
}
