import { useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { Login } from "@/components/auth/login";
import { openApiQueryOptions } from "@/generated/openapi-query-options";

export const Route = createFileRoute("/login")({
  component: LoginRoute,
});

function LoginRoute() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const meQueryOptions = openApiQueryOptions.getMe();

  return (
    <Login
      onLogin={async () => {
        await queryClient.invalidateQueries({ queryKey: meQueryOptions.queryKey });
        await navigate({ to: "/workspace/repositories", replace: true });
      }}
    />
  );
}
