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
    <div className="grid min-h-screen w-full place-items-center bg-[radial-gradient(circle_at_50%_0%,color-mix(in_oklch,var(--accent)_12%,transparent),transparent_38%)] px-4 py-10">
      <Card className="w-full max-w-[380px] border-border/90 bg-card/95 text-card-foreground shadow-2xl shadow-black/35">
        <CardHeader className="gap-5 pb-2">
          <div className="flex items-center justify-between">
            <div className="brand p-0"><Logo size={38} /><span className="brand-text">fork<span>lift</span></span></div>
            <span className="rounded-full border border-border bg-muted/50 px-2.5 py-1 text-[11px] font-medium text-muted-foreground">
              secure
            </span>
          </div>
          <div className="space-y-1">
            <CardTitle className="text-xl">Sign in</CardTitle>
            <CardDescription className="leading-relaxed">Access repository proxy controls, approvals, and personal tokens.</CardDescription>
          </div>
        </CardHeader>
        <CardContent className="pt-2">
          <form onSubmit={submit}>
            <FieldGroup>
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor="username">Username<span className="req">*</span></FieldLabel>
                <Input
                  id="username"
                  className="h-10"
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
                  className="h-10"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                  required
                  aria-invalid={Boolean(error)}
                />
                {error && <FieldError>{error}</FieldError>}
              </Field>
              <Button className="h-10 w-full" disabled={busy} type="submit">
                {busy ? "Signing in…" : "Sign in"}
              </Button>
            </FieldGroup>
          </form>
          {oidcEnabled && <FieldSeparator className="my-5">or</FieldSeparator>}
          {oidcEnabled && (
            <a
              className={cn(buttonVariants({ variant: "outline", size: "lg" }), "h-10 w-full")}
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
