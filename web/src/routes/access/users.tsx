import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/access/users")({
  component: Outlet,
});
