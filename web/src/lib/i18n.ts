import { useSyncExternalStore } from "react";
import {
  languagePreferenceChangedEvent,
  readLanguage,
  type Language,
} from "@/lib/user-preferences";
import en from "@/locales/en.json";
import ko from "@/locales/ko.json";

const messages = { en, ko } satisfies Record<Language, typeof en>;

type MessageKey = keyof typeof messages.en;

export function useLanguage() {
  return useSyncExternalStore(subscribeLanguage, readLanguage, () => "en" as Language);
}

export function useTranslation() {
  const language = useLanguage();
  const t = (key: MessageKey) => messages[language][key] ?? messages.en[key];
  return { language, t };
}

function subscribeLanguage(callback: () => void) {
  const onStorage = (event: StorageEvent) => {
    if (event.key === "forklift.language") callback();
  };

  window.addEventListener(languagePreferenceChangedEvent, callback);
  window.addEventListener("storage", onStorage);

  return () => {
    window.removeEventListener(languagePreferenceChangedEvent, callback);
    window.removeEventListener("storage", onStorage);
  };
}
