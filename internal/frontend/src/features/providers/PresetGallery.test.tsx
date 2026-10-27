import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nextProvider } from "react-i18next";
import i18n from "../../i18n";
import { Preset } from "../../presets/presets";
import { PresetGallery } from "./PresetGallery";

vi.mock("../../presets/presets", async () => {
  const actual =
    await vi.importActual<typeof import("../../presets/presets")>("../../presets/presets");
  return {
    ...actual,
    fetchPresets: vi.fn().mockResolvedValue({
      presets: [
        {
          id: "deepseek",
          displayName: "DeepSeek",
          category: "cn_official",
          apiFormat: "anthropic",
          requiredVars: ["ANTHROPIC_AUTH_TOKEN"],
          optionalVars: [],
          oauth: false,
        },
        {
          id: "openrouter",
          displayName: "OpenRouter",
          category: "aggregator",
          apiFormat: "anthropic",
          requiredVars: ["ANTHROPIC_AUTH_TOKEN"],
          optionalVars: [],
          oauth: false,
        },
      ] satisfies Preset[],
      categories: ["cn_official", "aggregator"],
    }),
  };
});

function renderGallery(onPick: (id: string) => void) {
  return render(
    <I18nextProvider i18n={i18n}>
      <PresetGallery open onPick={onPick} onClose={vi.fn()} />
    </I18nextProvider>,
  );
}

describe("PresetGallery（US3 / FR-009）", () => {
  it("按分类分组渲染预设卡片与自定义入口", async () => {
    renderGallery(vi.fn());
    await waitFor(() => {
      expect(screen.getByTestId("preset-gallery")).toBeTruthy();
    });
    expect(screen.getByTestId("preset-gallery-item-deepseek")).toBeTruthy();
    expect(screen.getByTestId("preset-gallery-item-openrouter")).toBeTruthy();
    expect(screen.getByTestId("preset-gallery-item-custom")).toBeTruthy();
  });

  it("点击卡片回调 onPick 并带出预设 id", async () => {
    const onPick = vi.fn();
    renderGallery(onPick);
    await waitFor(() => {
      expect(screen.getByTestId("preset-gallery-item-deepseek")).toBeTruthy();
    });
    fireEvent.click(screen.getByTestId("preset-gallery-item-deepseek"));
    expect(onPick).toHaveBeenCalledWith("deepseek");
  });

  it("自定义入口回调 custom", async () => {
    const onPick = vi.fn();
    renderGallery(onPick);
    await waitFor(() => {
      expect(screen.getByTestId("preset-gallery-item-custom")).toBeTruthy();
    });
    fireEvent.click(screen.getByTestId("preset-gallery-item-custom"));
    expect(onPick).toHaveBeenCalledWith("custom");
  });
});
