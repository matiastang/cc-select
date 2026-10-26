import { useTranslation } from "react-i18next";
import { Boxes } from "lucide-react";
import { Provider } from "../../types";
import { OFFICIAL_ID } from "../../constants";
import { Card, EmptyState } from "../../components/ui";
import { ProviderCard } from "./ProviderCard";
import { JsonForm } from "./JsonForm";

type ProviderListProps = {
  providers: Provider[];
  editingId: string | null;
  onEditStart: (id: string) => void;
  onEditCancel: () => void;
  onDelete: (id: string) => void;
  onSaved: () => void;
  onAdd?: () => void; // 空状态 CTA（FR-007）：引导从预设画廊开始
};

export function ProviderList({
  providers,
  editingId,
  onEditStart,
  onEditCancel,
  onDelete,
  onSaved,
  onAdd,
}: ProviderListProps) {
  const { t } = useTranslation("providers");

  // 官方 provider 是内置默认（不可删、无需配置）；「真正配置过」以排除官方后
  // 是否还有服务商判断——仅有官方时同样显示引导（FR-007 的实际可达形态）。
  const meaningful = providers.filter((p) => p.id !== OFFICIAL_ID);

  const emptyState = meaningful.length === 0 && (
    <Card flat>
      <EmptyState
        data-testid="providers-empty-state"
        icon={<Boxes size={32} />}
        title={t("empty.title")}
        description={t("empty.description")}
        actionLabel={onAdd ? t("empty.action") : undefined}
        onAction={onAdd}
      />
    </Card>
  );

  return (
    <>
      {emptyState}
      {[...providers]
        .sort((a, b) => a.id.localeCompare(b.id))
        .map((provider) => (
          <Card key={provider.id}>
            {editingId === provider.id ? (
              <JsonForm mode="edit" id={provider.id} onCancel={onEditCancel} onSaved={onSaved} />
            ) : (
              <ProviderCard provider={provider} onEdit={onEditStart} onDelete={onDelete} />
            )}
          </Card>
        ))}
    </>
  );
}
