import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Pencil, Sparkles } from "lucide-react";
import {
  Badge,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Spinner,
} from "../../components/ui";
import {
  CUSTOM_PRESET_ID,
  Preset,
  PresetCategory,
  PresetsMeta,
  fetchPresets,
  groupPresetsByCategory,
} from "../../presets/presets";

type PresetGalleryProps = {
  open: boolean;
  onPick: (presetId: string) => void;
  onClose: () => void;
};

// 预设画廊（US3 / FR-009）：分类分组的服务商卡片 + 自定义入口。
// 选择即进入向导——第一步通常只需 API key。
export function PresetGallery({ open, onPick, onClose }: PresetGalleryProps) {
  const { t } = useTranslation("providers");
  const [presets, setPresets] = useState<Preset[]>([]);
  const [categories, setCategories] = useState<PresetCategory[]>([]);
  const [meta, setMeta] = useState<PresetsMeta | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    let cancelled = false;
    void fetchPresets()
      .then((r) => {
        if (cancelled) return;
        setPresets(r.presets);
        setCategories(r.categories);
        setMeta(r.meta ?? null);
      })
      .catch(() => {
        if (!cancelled) {
          setPresets([]);
          setCategories([]);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open]);

  const groups = groupPresetsByCategory(presets, categories);

  return (
    <Dialog open={open} onOpenChange={(next) => (!next ? onClose() : undefined)}>
      <DialogContent className="w-[min(94vw,680px)]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {t("gallery.title")}
            {meta && (
              <Badge
                variant={meta.source === "remote" ? "success" : "muted"}
                data-testid="presets-source-badge"
              >
                {t(`gallery.source.${meta.source}`)}
              </Badge>
            )}
          </DialogTitle>
          <DialogDescription>{t("gallery.subtitle")}</DialogDescription>
        </DialogHeader>

        <div data-testid="preset-gallery" className="flex flex-col gap-5">
          {loading && (
            <div className="flex items-center gap-2 text-sm text-muted">
              <Spinner label={t("gallery.loading")} />
              {t("gallery.loading")}
            </div>
          )}
          {!loading &&
            categories.map((cat) => {
              // 后端预设表自带 custom 条目——由底部专用自定义入口承载，此处去重。
              const items = (groups[cat] ?? []).filter((p) => p.id !== CUSTOM_PRESET_ID);
              if (items.length === 0) return null;
              return (
                <section key={cat} data-testid={`preset-gallery-group-${cat}`}>
                  <Badge variant="muted" className="mb-2">
                    {t(`presetCategory.${cat}`)}
                  </Badge>
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                    {items.map((p) => (
                      <button
                        key={p.id}
                        type="button"
                        data-testid={`preset-gallery-item-${p.id}`}
                        onClick={() => onPick(p.id)}
                        className="flex flex-col items-start gap-1 rounded-md border border-border bg-card px-3 py-2.5 text-left transition-colors hover:border-accent hover:bg-bg"
                      >
                        <span className="flex items-center gap-1.5 text-sm font-medium text-text">
                          {p.id === "claude-official" ? (
                            <Sparkles size={14} className="text-accent" />
                          ) : null}
                          {p.displayName}
                          {p.oauth && <Badge variant="default">OAuth</Badge>}
                        </span>
                        {p.apiKeyURL && <span className="text-xs text-muted">{p.apiKeyURL}</span>}
                      </button>
                    ))}
                  </div>
                </section>
              );
            })}
          {!loading && (
            <section data-testid="preset-gallery-group-custom">
              <Badge variant="muted" className="mb-2">
                {t("presetCategory.custom")}
              </Badge>
              <button
                type="button"
                data-testid={`preset-gallery-item-${CUSTOM_PRESET_ID}`}
                onClick={() => onPick(CUSTOM_PRESET_ID)}
                className="flex w-full flex-col items-start gap-1 rounded-md border border-dashed border-border bg-card px-3 py-2.5 text-left transition-colors hover:border-accent hover:bg-bg"
              >
                <span className="flex items-center gap-1.5 text-sm font-medium text-text">
                  <Pencil size={14} className="text-muted" />
                  {t("gallery.customName")}
                </span>
                <span className="text-xs text-muted">{t("gallery.customDesc")}</span>
              </button>
            </section>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
