import { createFileRoute } from "@tanstack/react-router";
import { useAuth } from "../App";
import { RepositoryDetail } from "../pages/RepositoryDetail";

export const Route = createFileRoute("/repositories/$id")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  const { id } = Route.useParams();
  return <RepositoryDetail me={me} id={id} />;
}
