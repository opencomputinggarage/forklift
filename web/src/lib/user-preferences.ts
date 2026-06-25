export type ThemeMode = "system" | "dark" | "light";
export type Language = "en" | "ko";

const themeStorageKey = "forklift.theme";
const languageStorageKey = "forklift.language";
export const languagePreferenceChangedEvent = "forklift:language-change";

const themeModes: ThemeMode[] = ["system", "dark", "light"];
const languages: Language[] = ["en", "ko"];

export function readThemeMode(): ThemeMode {
  if (typeof window === "undefined") return "system";

  const stored = window.localStorage.getItem(themeStorageKey);
  return themeModes.includes(stored as ThemeMode) ? (stored as ThemeMode) : "system";
}

export function saveThemeMode(theme: ThemeMode) {
  window.localStorage.setItem(themeStorageKey, theme);
  applyThemeMode(theme);
}

export function readLanguage(): Language {
  if (typeof window === "undefined") return "en";

  const stored = window.localStorage.getItem(languageStorageKey);
  return languages.includes(stored as Language) ? (stored as Language) : "en";
}

export function saveLanguage(language: Language) {
  window.localStorage.setItem(languageStorageKey, language);
  applyLanguage(language);
  window.dispatchEvent(new Event(languagePreferenceChangedEvent));
}

export function applyUserPreferences() {
  applyThemeMode(readThemeMode());
  applyLanguage(readLanguage());
}

export function bindUserPreferenceListeners() {
  const unsubscribeTheme = subscribeToSystemTheme(() => {
    if (readThemeMode() === "system") applyThemeMode("system");
  });

  const onStorage = (event: StorageEvent) => {
    if (event.key === themeStorageKey) applyThemeMode(readThemeMode());
    if (event.key === languageStorageKey) applyLanguage(readLanguage());
  };

  window.addEventListener("storage", onStorage);

  return () => {
    unsubscribeTheme();
    window.removeEventListener("storage", onStorage);
  };
}

export function applyThemeMode(theme: ThemeMode) {
  if (typeof window === "undefined") return;

  const resolved = resolveThemeMode(theme);
  document.documentElement.classList.toggle("dark", resolved === "dark");
  document.documentElement.classList.toggle("light", resolved === "light");
}

export function applyLanguage(language: Language) {
  if (typeof document === "undefined") return;

  document.documentElement.lang = language;
}

export function resolveThemeMode(theme: ThemeMode): Exclude<ThemeMode, "system"> {
  if (theme !== "system") return theme;

  return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

export function subscribeToSystemTheme(callback: () => void) {
  const media = window.matchMedia("(prefers-color-scheme: light)");
  media.addEventListener("change", callback);
  return () => media.removeEventListener("change", callback);
}
