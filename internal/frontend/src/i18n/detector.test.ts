import { describe, it, expect } from "vitest";
import { mapDetectedLanguage } from "./detector";

describe("mapDetectedLanguage", () => {
  it("maps Chinese variants to zh", () => {
    expect(mapDetectedLanguage("zh")).toBe("zh");
    expect(mapDetectedLanguage("zh-CN")).toBe("zh");
    expect(mapDetectedLanguage("zh-Hans")).toBe("zh");
    expect(mapDetectedLanguage("zh-SG")).toBe("zh");
    expect(mapDetectedLanguage("zh-TW")).toBe("zh");
  });

  it("maps English variants to en", () => {
    expect(mapDetectedLanguage("en")).toBe("en");
    expect(mapDetectedLanguage("en-US")).toBe("en");
    expect(mapDetectedLanguage("en-GB")).toBe("en");
  });

  it("falls back to en for unsupported languages", () => {
    expect(mapDetectedLanguage("fr")).toBe("en");
    expect(mapDetectedLanguage("de")).toBe("en");
    expect(mapDetectedLanguage("ja-JP")).toBe("en");
  });
});
