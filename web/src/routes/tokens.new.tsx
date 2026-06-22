import { createFileRoute } from "@tanstack/react-router";
import { TokenNew } from "../pages/TokenNew";

export const Route = createFileRoute("/tokens/new")({
  component: TokenNew,
});
