import { createFileRoute } from "@tanstack/react-router";
import { useAuth } from "../../../authContext";
import { RepositoryDetail } from "./index";

export const Route = createFileRoute("/repositories/$id/$tab")({
  component: RepositoryDetailRoute,
});

function RepositoryDetailRoute() {
  const { me } = useAuth();
  return <RepositoryDetail me={me} />;
}
