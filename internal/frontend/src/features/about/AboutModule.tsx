import { Trans, useTranslation } from "react-i18next";
import { Card } from "../../components/ui";
import { IdPlaceholder } from "../../components/IdPlaceholder";

// 「关于」模块：产品简介与使用说明（承自旧 App 的 subtitle/notice 区）。
export function AboutModule() {
  const { t } = useTranslation();
  return (
    <div data-testid="module-about" className="flex flex-col gap-4">
      <Card>
        <h2 className="mb-1 text-base font-semibold">{t("aboutTitle")}</h2>
        <div className="notice">
          <Trans
            i18nKey="notice"
            ns="common"
            components={{ code: <code />, strong: <strong />, id: <IdPlaceholder /> }}
          />
        </div>
      </Card>
    </div>
  );
}
