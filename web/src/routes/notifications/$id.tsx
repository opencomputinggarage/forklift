import { createFileRoute, Navigate, useParams } from "@tanstack/react-router";
import { useAuth } from "@/authContext";
import { ReceiverForm } from "./new";

export const Route = createFileRoute("/notifications/$id")({
  component: ReceiverEditRoute,
});

function ReceiverEditRoute() {
  const { me } = useAuth();
  const { id } = useParams({ from: "/notifications/$id" });
  if (!me.admin) return <Navigate to="/repositories" replace />;
  return <ReceiverForm receiverId={Number(id)} />;
}
