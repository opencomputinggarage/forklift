import { useState, type ReactNode } from "react";
import { createFileRoute } from "@tanstack/react-router";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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

export const Route = createFileRoute("/settings")({
  component: SettingsRoute,
});

function SettingsRoute() {
  const { t } = useTranslation();
  const [theme, setTheme] = useState<ThemeMode>(() => readThemeMode());
  const [language, setLanguage] = useState<Language>(() => readLanguage());
  const themeOptions = [
    { value: "system", label: t("settings.theme.system"), description: t("settings.theme.systemDescription") },
    { value: "dark", label: t("settings.theme.dark"), description: t("settings.theme.darkDescription") },
    { value: "light", label: t("settings.theme.light"), description: t("settings.theme.lightDescription") },
  ] satisfies { value: ThemeMode; label: string; description: string }[];
  const languageOptions = [
    { value: "en", label: t("settings.language.en"), description: t("settings.language.enDescription") },
    { value: "ko", label: t("settings.language.ko"), description: t("settings.language.koDescription") },
  ] satisfies { value: Language; label: string; description: string }[];

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

      <Card className="border-border/90 bg-card/95 shadow-none">
        <CardHeader>
          <CardTitle>{t("settings.cardTitle")}</CardTitle>
          <CardDescription>
            {t("settings.cardDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent>
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
                    <span className="flex min-w-0 flex-col">
                      <span>{option.label}</span>
                      <span className="text-xs leading-4 text-muted-foreground">
                        {option.description}
                      </span>
                    </span>
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
                    <span className="flex min-w-0 flex-col">
                      <span>{option.label}</span>
                      <span className="text-xs leading-4 text-muted-foreground">
                        {option.description}
                      </span>
                    </span>
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
