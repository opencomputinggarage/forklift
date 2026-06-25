import { ComponentProps, FormEvent, useEffect, useState } from "react";
import { KeyRound } from "lucide-react";
import { api } from "@/api";
import loginBackgroundUrl from "@/assets/login-cyber-forklift.png";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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
      <img
        src={loginBackgroundUrl}
        alt=""
        className="pointer-events-none absolute inset-0 z-0 size-full scale-[1.045] object-cover object-[35%_50%] blur-[5px] brightness-[1.04] contrast-[1.14] saturate-[1.28]"
      />
      <div className="pointer-events-none absolute inset-0 z-0 bg-[radial-gradient(circle_at_28%_63%,rgba(21,230,255,0.24)_0%,transparent_23%),radial-gradient(circle_at_72%_30%,rgba(255,43,214,0.16)_0%,transparent_26%),linear-gradient(90deg,rgba(5,5,6,0.08)_0%,rgba(5,5,6,0.16)_42%,rgba(5,5,6,0.32)_72%,rgba(5,5,6,0.58)_100%)] mix-blend-screen" />
      <div className="pointer-events-none absolute inset-0 z-0 bg-[radial-gradient(ellipse_34%_48%_at_50%_50%,rgba(5,5,6,0.88)_0%,rgba(5,5,6,0.72)_34%,transparent_68%),linear-gradient(180deg,rgba(5,5,6,0.04)_0%,rgba(5,5,6,0.2)_58%,rgba(5,5,6,0.66)_100%)]" />
      <div className="pointer-events-none absolute inset-0 z-0 bg-[linear-gradient(rgba(21,230,255,0.26)_1px,transparent_1px),linear-gradient(90deg,rgba(255,43,214,0.16)_1px,transparent_1px)] bg-[length:52px_52px] opacity-[0.26] mix-blend-screen [animation:login-grid-drift_30s_linear_infinite] [mask-image:linear-gradient(90deg,transparent_0%,rgba(0,0,0,0.72)_38%,#000_100%)] motion-reduce:animate-none" />
      <div className="pointer-events-none absolute inset-x-0 bottom-0 z-0 h-[46%] bg-[linear-gradient(rgba(21,230,255,0.34)_1px,transparent_1px),linear-gradient(90deg,rgba(21,230,255,0.2)_1px,transparent_1px)] bg-[length:72px_34px] opacity-[0.34] mix-blend-screen [transform:perspective(680px)_rotateX(58deg)_translateY(28%)] [transform-origin:50%_100%] [mask-image:linear-gradient(180deg,transparent_0%,#000_34%,#000_88%,transparent_100%)]" />
      <div className="pointer-events-none absolute inset-0 z-0 bg-[radial-gradient(circle_at_50%_50%,transparent_0%,transparent_42%,rgba(0,0,0,0.62)_100%)]" />
      <div className="pointer-events-none absolute inset-x-0 top-0 z-0 h-40 border-b border-cyan-300/15 bg-gradient-to-b from-black/28 to-transparent" />
      <div className="relative z-10 flex w-full max-w-[min(24rem,calc(100vw-2rem))] min-w-0 flex-col gap-6">
        <div className="flex min-w-0 items-center justify-center drop-shadow-[0_0_28px_rgba(21,230,255,0.48)]">
          <span className="truncate text-[clamp(3.5rem,15vw,5.5rem)] font-bold leading-none tracking-normal text-foreground">
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
      <Card className="w-full min-w-0 border-cyan-300/25 bg-card/88 text-card-foreground shadow-2xl shadow-black/60 backdrop-blur-xl">
        <CardContent className="px-5 py-5 sm:px-6 sm:py-6">
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
