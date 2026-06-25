import { createFileRoute, Navigate, useParams } from "@tanstack/react-router";

export const Route = createFileRoute("/notifications/$id")({
  component: ReceiverEditRoute,
});

function ReceiverEditRoute() {
  const { id } = useParams({ from: "/notifications/$id" });
  return <Navigate to="/admin/notifications/$id" params={{ id }} replace />;
}
