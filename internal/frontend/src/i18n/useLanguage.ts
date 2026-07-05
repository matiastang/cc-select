import { useTranslation } from "react-i18next";
import { SupportedLocale } from "./config";

export function useLanguage() {
  const { i18n } = useTranslation();
  return {
    language: i18n.language as SupportedLocale,
    changeLanguage: (lng: SupportedLocale) => i18n.changeLanguage(lng),
  };
}
