import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "@/authContext";
import { TokenNew } from "../../../../workspace/tokens/new";

export const Route = createFileRoute("/access/users/$id/tokens/new")({
  component: UserTokenNewRoute,
});

function UserTokenNewRoute() {
  const { me } = useAuth();
  return me.admin ? <TokenNew /> : <Navigate to="/workspace/repositories" replace />;
}
