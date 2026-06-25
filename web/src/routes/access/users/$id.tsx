import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/access/users/$id")({
  component: Outlet,
});
