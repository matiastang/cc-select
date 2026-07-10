import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { I18nextProvider } from "react-i18next";
import i18n from "../i18n";
import { ThemeSwitcher } from "./ThemeSwitcher";

function renderWithI18n(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}

describe("ThemeSwitcher", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  afterEach(() => {
    document.documentElement.removeAttribute("data-theme");
  });

  it("默认高亮 system 按钮", () => {
    renderWithI18n(<ThemeSwitcher />);
    expect(screen.getByTestId("theme-system")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("theme-light")).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByTestId("theme-dark")).toHaveAttribute("aria-pressed", "false");
  });

  it("点击 light 会设置 data-theme 为 light", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ThemeSwitcher />);
    await user.click(screen.getByTestId("theme-light"));
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(localStorage.getItem("cc-select-theme")).toBe("light");
  });

  it("点击 dark 会设置 data-theme 为 dark", async () => {
    const user = userEvent.setup();
    renderWithI18n(<ThemeSwitcher />);
    await user.click(screen.getByTestId("theme-dark"));
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(localStorage.getItem("cc-select-theme")).toBe("dark");
  });

  it("点击 system 会移除 data-theme", async () => {
    document.documentElement.setAttribute("data-theme", "dark");
    localStorage.setItem("cc-select-theme", "dark");
    const user = userEvent.setup();
    renderWithI18n(<ThemeSwitcher />);
    await user.click(screen.getByTestId("theme-system"));
    expect(document.documentElement.getAttribute("data-theme")).toBeNull();
    expect(localStorage.getItem("cc-select-theme")).toBe("system");
  });
});
