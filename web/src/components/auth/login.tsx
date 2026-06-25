import { ComponentProps, FormEvent, useEffect, useState } from "react";
import { KeyRound } from "lucide-react";
import { api } from "@/api";
import { Logo } from "@/components/app/logo";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export function Login({ onLogin }: { onLogin: () => void | Promise<void> }) {
  return (
    <div className="relative isolate grid min-h-svh w-full place-items-center overflow-hidden bg-background px-4 py-8 sm:px-6">
      <div className="login-bg-base pointer-events-none absolute inset-0 z-0" />
      <div className="login-bg-grid pointer-events-none absolute inset-0 z-0" />
      <div className="login-bg-wave pointer-events-none absolute inset-0 z-0" />
      <div className="pointer-events-none absolute inset-x-0 top-0 z-0 h-48 border-b border-border/35" />
      <div className="pointer-events-none absolute left-1/2 top-1/2 z-0 h-[32rem] w-[32rem] -translate-x-1/2 -translate-y-1/2 rounded-full border border-primary/10 bg-primary/[0.035] blur-3xl" />
      <div className="relative z-10 flex w-full max-w-[24rem] flex-col gap-5">
        <div className="flex min-w-0 items-center justify-center gap-3">
          <Logo size={32} />
          <span className="truncate text-xl font-bold tracking-normal text-foreground">
            fork<span className="text-primary">lift</span>
          </span>
        </div>
        <LoginForm onLogin={onLogin} />
      </div>
    </div>
  );
}

function LoginForm({
  onLogin,
  className,
  ...props
}: ComponentProps<"div"> & { onLogin: () => void | Promise<void> }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // Only offer Keycloak when OIDC is configured; otherwise /auth/login 404s.
  const [oidcEnabled, setOidcEnabled] = useState(false);

  useEffect(() => {
    api
      .version()
      .then((v) => setOidcEnabled(v.oidc_enabled))
      .catch(() => setOidcEnabled(false));
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await api.login(username, password);
      await onLogin();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <Card className="border-border/90 bg-card/95 text-card-foreground shadow-2xl shadow-black/30 backdrop-blur">
        <CardHeader className="gap-2 px-5 pt-5 sm:px-6 sm:pt-6">
          <CardTitle className="text-2xl font-semibold tracking-normal">
            Sign in to forklift
          </CardTitle>
          <CardDescription className="leading-6">
            Manage repository policies, approvals, and scoped tokens.
          </CardDescription>
        </CardHeader>
        <CardContent className="px-5 pb-5 sm:px-6 sm:pb-6">
          <form onSubmit={submit}>
            <FieldGroup className="gap-5">
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor="username">
                  Username
                </FieldLabel>
                <Input
                  id="username"
                  className="h-11 border-border bg-background/70"
                  placeholder="Username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoComplete="username"
                  autoFocus
                  required
                  aria-invalid={Boolean(error)}
                />
              </Field>
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor="password">
                  Password
                </FieldLabel>
                <Input
                  id="password"
                  className="h-11 border-border bg-background/70"
                  type="password"
                  placeholder="Password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                  required
                  aria-invalid={Boolean(error)}
                />
                {error && <FieldError>{error}</FieldError>}
              </Field>
              <Field className="gap-3 pt-1">
                <Button className="h-11 w-full" disabled={busy} type="submit">
                  {busy ? (
                    "Signing in..."
                  ) : (
                    <>
                      <KeyRound data-icon="inline-start" aria-hidden="true" />
                      Sign in
                    </>
                  )}
                </Button>
                {oidcEnabled && (
                  <Button
                    render={<a href="/auth/login" />}
                    nativeButton={false}
                    variant="outline"
                    className="h-11 w-full border-border bg-background/70"
                  >
                    Sign in with Keycloak
                  </Button>
                )}
                <FieldDescription className="text-center">
                  Access is limited by your assigned role.
                </FieldDescription>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
