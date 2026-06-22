import { createFileRoute } from "@tanstack/react-router";
import { Tokens } from "../pages/Tokens";

export const Route = createFileRoute("/tokens")({
  component: Tokens,
});
