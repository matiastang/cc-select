import { describe, it, expect, beforeEach } from "vitest";
import { Theme, getStoredTheme, setStoredTheme, applyTheme } from "./theme";

describe("theme", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
  });

  it("未保存过主题时默认返回 system", () => {
    expect(getStoredTheme()).toBe("system");
  });

  it("能正确读写 localStorage", () => {
    setStoredTheme("dark");
    expect(getStoredTheme()).toBe("dark");
    expect(localStorage.getItem("cc-select-theme")).toBe("dark");
  });

  it("非法值会被忽略，回退到 system", () => {
    localStorage.setItem("cc-select-theme", "invalid");
    expect(getStoredTheme()).toBe("system");
  });

  it("applyTheme system 会移除 data-theme 属性", () => {
    document.documentElement.setAttribute("data-theme", "dark");
    applyTheme("system");
    expect(document.documentElement.getAttribute("data-theme")).toBeNull();
  });

  it.each<Theme>(["light", "dark"])("applyTheme %s 会设置 data-theme", (theme) => {
    applyTheme(theme);
    expect(document.documentElement.getAttribute("data-theme")).toBe(theme);
  });
});
