import { describe, it, expect, beforeEach } from "vitest";
import i18n from "./index";
import { resources } from "./locales";
import { SUPPORTED_LOCALES, DEFAULT_LOCALE } from "./config";

function collectKeys(obj: Record<string, unknown>, prefix = ""): string[] {
  const keys: string[] = [];
  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      keys.push(...collectKeys(value as Record<string, unknown>, fullKey));
    } else {
      keys.push(fullKey);
    }
  }
  return keys.sort();
}

describe("i18n setup", () => {
  beforeEach(async () => {
    await i18n.changeLanguage(DEFAULT_LOCALE);
  });

  it("has the same keys in every locale namespace", () => {
    for (const ns of ["common", "providers", "shell"] as const) {
      const baseKeys = collectKeys(resources[DEFAULT_LOCALE][ns]);
      for (const lng of SUPPORTED_LOCALES) {
        if (lng === DEFAULT_LOCALE) continue;
        const localeKeys = collectKeys(resources[lng][ns]);
        expect(localeKeys).toEqual(baseKeys);
      }
    }
  });

  it("falls back to English for unsupported languages", async () => {
    await i18n.changeLanguage("fr");
    expect(i18n.language).toBe(DEFAULT_LOCALE);
  });
});
