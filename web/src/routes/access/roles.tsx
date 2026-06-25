import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/access/roles")({
  component: Outlet,
});
