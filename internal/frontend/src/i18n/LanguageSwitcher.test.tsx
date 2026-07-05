import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import i18n from "./index";
import { LanguageSwitcher } from "./LanguageSwitcher";

function renderWithI18n(ui: React.ReactNode) {
  return render(<I18nextProvider i18n={i18n}>{ui}</I18nextProvider>);
}

describe("LanguageSwitcher", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("zh");
  });

  it("renders all supported locales", () => {
    renderWithI18n(<LanguageSwitcher />);
    expect(screen.getByRole("button", { name: "简体中文" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "English" })).toBeInTheDocument();
  });

  it("highlights the active locale", () => {
    renderWithI18n(<LanguageSwitcher />);
    expect(screen.getByRole("button", { name: "简体中文" })).toHaveClass("active");
    expect(screen.getByRole("button", { name: "English" })).not.toHaveClass("active");
  });

  it("switches language when clicked", () => {
    renderWithI18n(<LanguageSwitcher />);
    fireEvent.click(screen.getByRole("button", { name: "English" }));
    expect(i18n.language).toBe("en");
  });
});
