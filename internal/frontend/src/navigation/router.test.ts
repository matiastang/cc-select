import { renderHook, act } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DEFAULT_ROUTE, navigate, parseHash, useHashRoute } from "./router";

describe("parseHash", () => {
  it("normalizes empty and bare hashes to default route", () => {
    expect(parseHash("")).toBe(DEFAULT_ROUTE);
    expect(parseHash("#")).toBe(DEFAULT_ROUTE);
    expect(parseHash("#/")).toBe(DEFAULT_ROUTE);
    expect(parseHash("#/unknown-route-xyz")).toBe(DEFAULT_ROUTE);
  });

  it("keeps known routes", () => {
    expect(parseHash("#/providers")).toBe("#/providers");
    expect(parseHash("#/mode-routes")).toBe("#/mode-routes");
  });

  it("trims trailing slash", () => {
    expect(parseHash("#/providers/")).toBe("#/providers");
  });
});

describe("navigate", () => {
  it("sets location.hash", () => {
    navigate("#/about");
    expect(window.location.hash).toBe("#/about");
  });
});

describe("useHashRoute", () => {
  it("follows hashchange and normalizes unknown to default", () => {
    window.location.hash = "";
    const { result } = renderHook(() => useHashRoute());
    expect(result.current).toBe(DEFAULT_ROUTE);

    act(() => {
      navigate("#/security");
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(result.current).toBe("#/security");

    act(() => {
      navigate("#/nope");
      window.dispatchEvent(new HashChangeEvent("hashchange"));
    });
    expect(result.current).toBe(DEFAULT_ROUTE);
  });
});
