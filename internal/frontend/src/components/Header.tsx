import { useEffect } from "react";
import { Trans, useTranslation } from "react-i18next";
import { LanguageSwitcher } from "../i18n/LanguageSwitcher";
import { updateDocumentLang, updateDocumentTitle } from "../i18n/utils";

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
        <LanguageSwitcher />
      </div>
      <p className="muted">
        <Trans i18nKey="subtitle">
          管理各 AI 服务商配置。切换请在终端用 <code>ccs use <span>&lt;id&gt;</span></code>。
        </Trans>
      </p>
    </div>
  );
}
