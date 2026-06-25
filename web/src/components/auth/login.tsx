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
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

export function Login({ onLogin }: { onLogin: () => void | Promise<void> }) {
  return (
    <div className="relative isolate grid min-h-svh w-full place-items-center overflow-hidden bg-background px-4 py-8 sm:px-6">
      <div className="pointer-events-none absolute inset-0 z-0 bg-[radial-gradient(circle_at_50%_0%,color-mix(in_oklch,var(--fx-accent)_16%,transparent),transparent_36%),radial-gradient(circle_at_50%_44%,var(--fx-login-glow),transparent_34%),linear-gradient(180deg,var(--fx-body-gradient-start)_0%,var(--fx-canvas)_100%)] [animation:login-glow_9s_ease-in-out_infinite_alternate] motion-reduce:animate-none" />
      <div className="pointer-events-none absolute inset-0 z-0 bg-[linear-gradient(var(--border)_1px,transparent_1px),linear-gradient(90deg,var(--border)_1px,transparent_1px)] bg-[length:40px_40px] opacity-[0.14] [animation:login-grid-drift_28s_linear_infinite] [mask-image:radial-gradient(circle_at_50%_42%,#000_0%,#000_42%,transparent_78%)] motion-reduce:animate-none" />
      <div className="pointer-events-none absolute inset-0 z-0 bg-[radial-gradient(ellipse_60%_18%_at_50%_42%,color-mix(in_oklch,var(--fx-accent)_13%,transparent),transparent_62%),repeating-radial-gradient(ellipse_78%_18%_at_50%_42%,transparent_0_18px,var(--fx-login-wave)_19px_20px,transparent_22px_42px)] opacity-[0.34] mix-blend-screen [animation:login-wave_12s_ease-in-out_infinite] [mask-image:radial-gradient(ellipse_at_50%_42%,#000_0%,#000_32%,transparent_70%)] [transform:translate3d(0,0,0)] motion-reduce:animate-none" />
      <div className="pointer-events-none absolute inset-x-0 top-0 z-0 h-48 border-b border-border/35" />
      <div className="pointer-events-none absolute left-1/2 top-1/2 z-0 h-[32rem] w-[32rem] -translate-x-1/2 -translate-y-1/2 rounded-full border border-primary/10 bg-primary/[0.035] blur-3xl" />
      <div className="relative z-10 flex w-full max-w-[min(24rem,calc(100vw-2rem))] min-w-0 flex-col gap-5">
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
  const { t } = useTranslation();
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
      <Card className="w-full min-w-0 border-border/90 bg-card/95 text-card-foreground shadow-2xl shadow-black/30 backdrop-blur">
        <CardHeader className="gap-2 px-5 pt-5 sm:px-6 sm:pt-6">
          <CardTitle className="text-2xl font-semibold tracking-normal">
            {t("login.title")}
          </CardTitle>
          <CardDescription className="leading-6 break-words">
            {t("login.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="px-5 pb-5 sm:px-6 sm:pb-6">
          <form onSubmit={submit}>
            <FieldGroup className="gap-5">
              <Field data-invalid={Boolean(error)}>
                <FieldLabel htmlFor="username">
                  {t("login.username")}
                </FieldLabel>
                <Input
                  id="username"
                  className="h-11 border-border bg-background/70"
                  placeholder={t("login.username")}
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
                  {t("login.password")}
                </FieldLabel>
                <Input
                  id="password"
                  className="h-11 border-border bg-background/70"
                  type="password"
                  placeholder={t("login.password")}
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
                    t("login.signingIn")
                  ) : (
                    <>
                      <KeyRound data-icon="inline-start" aria-hidden="true" />
                      {t("login.signIn")}
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
                    {t("login.keycloak")}
                  </Button>
                )}
                <FieldDescription className="text-center">
                  {t("login.accessNote")}
                </FieldDescription>
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
