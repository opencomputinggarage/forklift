import { useEffect } from "react";
import { hasLiteralTranslation, translateLiteral } from "@/locales/literals";
import { useLanguage } from "@/lib/i18n";

const translatedAttributes = ["aria-label", "placeholder", "title"] as const;
const skippedTextParents = new Set(["CODE", "KBD", "PRE", "SCRIPT", "STYLE", "TEXTAREA"]);

const originalText = new WeakMap<Text, string>();

function translateTextNode(node: Text, language: ReturnType<typeof useLanguage>) {
  const parent = node.parentElement;
  if (!parent || skippedTextParents.has(parent.tagName)) return;

  const current = node.nodeValue ?? "";
  const source = originalText.get(node) ?? current;

  const trimmed = source.trim();
  if (!trimmed) return;
  if (!hasLiteralTranslation(trimmed)) return;
  if (!originalText.has(node)) originalText.set(node, source);

  const translated = translateLiteral(trimmed, language);

  const leading = source.match(/^\s*/)?.[0] ?? "";
  const trailing = source.match(/\s*$/)?.[0] ?? "";
  const next = language === "en" ? source : `${leading}${translated}${trailing}`;
  if (node.nodeValue !== next) node.nodeValue = next;
}

function translateElementAttributes(element: Element, language: ReturnType<typeof useLanguage>) {
  for (const attr of translatedAttributes) {
    const current = element.getAttribute(attr);
    if (!current) continue;

    const marker = `data-i18n-original-${attr}`;
    const source = element.getAttribute(marker) ?? current;
    if (!hasLiteralTranslation(source)) continue;
    if (!element.hasAttribute(marker)) element.setAttribute(marker, source);

    const next = translateLiteral(source, language);
    if (current !== next) element.setAttribute(attr, next);
  }
}

function translateTree(root: ParentNode, language: ReturnType<typeof useLanguage>) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  let node = walker.nextNode();
  while (node) {
    translateTextNode(node as Text, language);
    node = walker.nextNode();
  }

  if (root instanceof Element) translateElementAttributes(root, language);
  root.querySelectorAll?.("*").forEach((element) => translateElementAttributes(element, language));
}

export function I18nLiteralBridge() {
  const language = useLanguage();

  useEffect(() => {
    const root = document.getElementById("root");
    if (!root) return;

    translateTree(root, language);
    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (mutation.type === "characterData") {
          translateTextNode(mutation.target as Text, language);
          continue;
        }
        if (mutation.type === "attributes" && mutation.target instanceof Element) {
          translateElementAttributes(mutation.target, language);
          continue;
        }
        mutation.addedNodes.forEach((node) => {
          if (node.nodeType === Node.TEXT_NODE) translateTextNode(node as Text, language);
          else if (node instanceof Element) translateTree(node, language);
        });
      }
    });

    observer.observe(root, {
      subtree: true,
      childList: true,
      characterData: true,
      attributes: true,
      attributeFilter: [...translatedAttributes],
    });

    return () => observer.disconnect();
  }, [language]);

  return null;
}
