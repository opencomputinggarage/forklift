import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/workspace/repositories/$id")({
  component: Outlet,
});
