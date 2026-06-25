import { useState, type ReactNode } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { Info } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Card, CardContent } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  readLanguage,
  readThemeMode,
  saveLanguage,
  saveThemeMode,
  type Language,
  type ThemeMode,
} from "@/lib/user-preferences";
import { useTranslation } from "@/lib/i18n";

export const Route = createFileRoute("/system/settings")({
  component: SettingsRoute,
});

export function SettingsRoute() {
  const { t } = useTranslation();
  const [theme, setTheme] = useState<ThemeMode>(() => readThemeMode());
  const [language, setLanguage] = useState<Language>(() => readLanguage());
  const themeOptions = [
    { value: "system", label: t("settings.theme.system") },
    { value: "dark", label: t("settings.theme.dark") },
    { value: "light", label: t("settings.theme.light") },
  ] satisfies { value: ThemeMode; label: string }[];
  const languageOptions = [
    { value: "en", label: t("settings.language.en") },
    { value: "ko", label: t("settings.language.ko") },
  ] satisfies { value: Language; label: string }[];

  const onThemeChange = (next: string) => {
    const nextTheme = next as ThemeMode;
    setTheme(nextTheme);
    saveThemeMode(nextTheme);
  };

  const onLanguageChange = (next: string) => {
    const nextLanguage = next as Language;
    setLanguage(nextLanguage);
    saveLanguage(nextLanguage);
  };

  return (
    <div className="max-w-[760px]">
      <div className="mb-5">
        <h1 className="m-0 text-2xl leading-tight font-semibold tracking-normal max-sm:text-xl">
          {t("settings.title")}
        </h1>
        <p className="mb-0 mt-2 text-sm leading-relaxed text-muted-foreground">
          {t("settings.description")}
        </p>
      </div>

      <Alert className="mb-4 border-border/80 bg-muted/40">
        <Info className="size-4" aria-hidden="true" />
        <AlertTitle>{t("settings.cardTitle")}</AlertTitle>
        <AlertDescription>{t("settings.cardDescription")}</AlertDescription>
      </Alert>

      <Card className="border-border/90 bg-card/95 shadow-none">
        <CardContent className="pt-6">
          <SettingRow
            id="appearance"
            title={t("settings.appearance")}
            description={t("settings.appearanceDescription")}
          >
            <Select
              items={themeOptions}
              value={theme}
              onValueChange={(next) => next && onThemeChange(next)}
            >
              <SelectTrigger id="appearance" className="w-full sm:w-[220px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="start">
                {themeOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>

          <Separator />

          <SettingRow
            id="language"
            title={t("settings.language")}
            description={t("settings.languageDescription")}
          >
            <Select
              items={languageOptions}
              value={language}
              onValueChange={(next) => next && onLanguageChange(next)}
            >
              <SelectTrigger id="language" className="w-full sm:w-[220px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="start">
                {languageOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>
        </CardContent>
      </Card>
    </div>
  );
}

function SettingRow({
  id,
  title,
  description,
  children,
}: {
  id: string;
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <div className="grid gap-3 py-4 first:pt-0 last:pb-0 sm:grid-cols-[1fr_240px] sm:items-center">
      <div className="min-w-0">
        <Label htmlFor={id}>{title}</Label>
        <p className="mb-0 mt-1 text-sm leading-relaxed text-muted-foreground">{description}</p>
      </div>
      <div className="min-w-0 sm:justify-self-end">{children}</div>
    </div>
  );
}
