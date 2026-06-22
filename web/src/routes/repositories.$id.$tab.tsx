import { createFileRoute } from "@tanstack/react-router";
import { useAuth } from "../App";
import { RepositoryDetail } from "../pages/RepositoryDetail";

export const Route = createFileRoute("/repositories/$id/$tab")({
  component: RouteComponent,
});

function RouteComponent() {
  const { me } = useAuth();
  const { id, tab } = Route.useParams();
  return <RepositoryDetail me={me} id={id} tab={tab} />;
}
