import en from "@/locales/en.json";
import ko from "@/locales/ko.json";
import type { Language } from "@/lib/user-preferences";

const messages: Record<Language, Record<string, string>> = { en, ko };

export function translateLiteral(value: string, language: Language): string {
  return messages[language][value] ?? value;
}

export function hasLiteralTranslation(value: string): boolean {
  return Object.prototype.hasOwnProperty.call(en, value);
}
