import { useTranslation } from "react-i18next";
import { Card } from "../../components/ui";
import { LanguageSwitcher } from "../../i18n/LanguageSwitcher";
import { ThemeSwitcher } from "../../theme/ThemeSwitcher";

// 「外观与通用」模块：主题/语言快速设置。远端预设卡片（US5）在 M2 落入此处。
export function GeneralModule() {
  const { t } = useTranslation("general");
  return (
    <div data-testid="module-general" className="flex flex-col gap-4">
      <Card>
        <h2 className="mb-1 text-base font-semibold">{t("theme.label")}</h2>
        <p className="mb-3 text-sm text-muted">{t("theme.helper")}</p>
        <ThemeSwitcher />
      </Card>
      <Card>
        <h2 className="mb-1 text-base font-semibold">{t("language.label")}</h2>
        <p className="mb-3 text-sm text-muted">{t("language.helper")}</p>
        <LanguageSwitcher />
      </Card>
    </div>
  );
}
