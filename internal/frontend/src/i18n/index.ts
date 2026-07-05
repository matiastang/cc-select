import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import { resources } from "./locales";
import { DEFAULT_LOCALE, SUPPORTED_LOCALES, STORAGE_KEY } from "./config";
import { mapDetectedLanguage } from "./detector";

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: DEFAULT_LOCALE,
    supportedLngs: SUPPORTED_LOCALES,
    defaultNS: "common",
    ns: ["common", "providers", "shell"],
    interpolation: {
      escapeValue: false, // React already escapes DOM output
    },
    detection: {
      order: ["localStorage", "navigator"],
      caches: ["localStorage"],
      lookupLocalStorage: STORAGE_KEY,
      convertDetectedLanguage: mapDetectedLanguage,
    },
    react: {
      useSuspense: false,
    },
  });

export default i18n;
