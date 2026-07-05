import { useCallback } from "react";
import { useTranslation } from "react-i18next";
import { LANGUAGE_API } from "../constants";
import { SupportedLocale, SUPPORTED_LOCALES, STORAGE_KEY } from "./config";

async function saveBackendLanguage(lang: SupportedLocale): Promise<void> {
  try {
    await fetch(LANGUAGE_API, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ language: lang }),
    });
  } catch {
    // Network down or server unreachable: localStorage remains as offline fallback.
  }
}

export async function fetchBackendLanguage(): Promise<SupportedLocale | null> {
  try {
    const r = await fetch(LANGUAGE_API);
    if (!r.ok) return null;
    const data = await r.json();
    const lang = data.language;
    if ((SUPPORTED_LOCALES as readonly string[]).includes(lang)) {
      return lang as SupportedLocale;
    }
  } catch {
    // Ignore network errors.
  }
  return null;
}

export function useLanguage() {
  const { i18n } = useTranslation();

  const changeLanguage = useCallback(
    async (lng: SupportedLocale) => {
      await i18n.changeLanguage(lng);
      localStorage.setItem(STORAGE_KEY, lng);
      await saveBackendLanguage(lng);
    },
    [i18n],
  );

  return {
    language: i18n.language as SupportedLocale,
    changeLanguage,
  };
}
