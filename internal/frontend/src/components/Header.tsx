import { useEffect } from "react";
import { Trans, useTranslation } from "react-i18next";
import { LanguageSwitcher } from "../i18n/LanguageSwitcher";
import { updateDocumentLang, updateDocumentTitle } from "../i18n/utils";
import { IdPlaceholder } from "./IdPlaceholder";
import { ThemeSwitcher } from "../theme/ThemeSwitcher";

export function Header() {
  const { t, i18n } = useTranslation();

  useEffect(() => {
    updateDocumentLang(i18n.language);
    updateDocumentTitle(t("appTitle"));
  }, [i18n.language, t]);

  return (
    <div className="header">
      <div className="header-title">
        <h1>{t("appTitle")}</h1>
        <div className="header-controls">
          <ThemeSwitcher />
          <LanguageSwitcher />
        </div>
      </div>
      <p className="muted">
        <Trans i18nKey="subtitle" components={{ code: <code />, id: <IdPlaceholder /> }} />
      </p>
    </div>
  );
}
