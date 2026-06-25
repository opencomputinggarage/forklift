import { createFileRoute, Navigate } from "@tanstack/react-router";
import { useAuth } from "../../../../authContext";
import { TokenNew } from "../../../tokens/new";

export const Route = createFileRoute("/users/$id/tokens/new")({
  component: UserTokenNewRoute,
});

function UserTokenNewRoute() {
  const { me } = useAuth();
  return me.admin ? <TokenNew /> : <Navigate to="/repositories" replace />;
}
