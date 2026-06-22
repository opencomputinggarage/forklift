import { FormEvent, useEffect, useState } from "react";
import { api } from "../api";
import { Logo } from "./Logo";
import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldSeparator,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export function Login({ onLogin }: { onLogin: () => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // Only offer Keycloak when OIDC is configured; otherwise /auth/login 404s.
  const [oidcEnabled, setOidcEnabled] = useState(false);

  useEffect(() => {
    api.version().then((v) => setOidcEnabled(v.oidc_enabled)).catch(() => setOidcEnabled(false));
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await api.login(username, password);
      onLogin();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-screen w-full items-center justify-center px-4">
      <Card className="w-full max-w-sm border-border bg-card text-card-foreground shadow-xl">
        <CardHeader className="gap-4">
          <div className="brand p-0"><Logo /><span className="brand-text">fork<span>lift</span></span></div>
          <div>
            <CardTitle>Sign in</CardTitle>
            <CardDescription>Access your repository proxy and package approvals.</CardDescription>
          </div>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit}>
            <FieldGroup>
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor="username">Username<span className="req">*</span></FieldLabel>
                <Input
                  id="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoComplete="username"
                  autoFocus
                  required
                  aria-invalid={Boolean(error)}
                />
              </Field>
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor="password">Password<span className="req">*</span></FieldLabel>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                  required
                  aria-invalid={Boolean(error)}
                />
                {error && <FieldError>{error}</FieldError>}
              </Field>
              <Button className="w-full" disabled={busy} type="submit">
                {busy ? "Signing in…" : "Sign in"}
              </Button>
            </FieldGroup>
          </form>
          {oidcEnabled && <FieldSeparator className="my-4">or</FieldSeparator>}
          {oidcEnabled && (
            <a
              className={cn(buttonVariants({ variant: "outline" }), "w-full")}
              href="/auth/login"
            >
              Sign in with Keycloak
            </a>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
