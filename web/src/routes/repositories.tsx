import { createFileRoute } from "@tanstack/react-router";
import { useAuth } from "../App";
import { Repositories } from "../pages/Repositories";

export const Route = createFileRoute("/repositories")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  return <Repositories me={me} />;
}
