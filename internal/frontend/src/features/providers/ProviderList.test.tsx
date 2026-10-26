import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nextProvider } from "react-i18next";
import i18n from "../../i18n";
import { Provider } from "../../types";
import { ProviderList } from "./ProviderList";

const baseProvider: Provider = {
  id: "deepseek",
  name: "DeepSeek",
  isolationMode: "",
  env: { ANTHROPIC_BASE_URL: "https://example.com" },
  hasKey: false,
  varKeys: [],
};

function renderList(props: Partial<Parameters<typeof ProviderList>[0]> = {}) {
  return render(
    <I18nextProvider i18n={i18n}>
      <ProviderList
        providers={[]}
        editingId={null}
        onEditStart={vi.fn()}
        onEditCancel={vi.fn()}
        onDelete={vi.fn()}
        onSaved={vi.fn()}
        {...props}
      />
    </I18nextProvider>,
  );
}

describe("ProviderList 空状态（FR-007）", () => {
  it("无 provider 时渲染空状态引导与添加动作", () => {
    const onAdd = vi.fn();
    renderList({ onAdd });
    expect(screen.getByTestId("providers-empty-state")).toBeTruthy();
    const cta = screen.getByRole("button", { name: /add/i });
    fireEvent.click(cta);
    expect(onAdd).toHaveBeenCalledOnce();
  });

  it("无 onAdd 时空状态不渲染 CTA 按钮", () => {
    renderList();
    expect(screen.getByTestId("providers-empty-state")).toBeTruthy();
    expect(screen.queryByRole("button", { name: /add/i })).toBeNull();
  });
});

describe("ProviderList 空状态（仅官方默认）", () => {
  it("只有官方 provider 时也显示引导", () => {
    renderList({
      providers: [{ ...baseProvider, id: "claude-official", name: "Claude Official" }],
      onAdd: vi.fn(),
    });
    expect(screen.getByTestId("providers-empty-state")).toBeTruthy();
    expect(screen.getByTestId("provider-card-claude-official")).toBeTruthy();
  });
});

describe("ProviderList 渲染", () => {
  it("按 id 排序渲染 provider 卡片", () => {
    renderList({
      providers: [
        { ...baseProvider, id: "zhipu", name: "GLM" },
        { ...baseProvider, id: "deepseek", name: "DeepSeek" },
      ],
    });
    const cards = screen.getAllByTestId(/^provider-card-/);
    expect(cards[0]).toHaveAttribute("data-testid", "provider-card-deepseek");
    expect(cards[1]).toHaveAttribute("data-testid", "provider-card-zhipu");
  });
});
