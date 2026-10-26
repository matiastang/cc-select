import { describe, expect, it } from "vitest";
import { buildNavModules } from "./registry";

const renderers = {
  providers: () => null as unknown as JSX.Element,
  modeRoutes: () => null as unknown as JSX.Element,
  security: () => null as unknown as JSX.Element,
  desktop: () => null as unknown as JSX.Element,
  general: () => null as unknown as JSX.Element,
  about: () => null as unknown as JSX.Element,
};

describe("buildNavModules", () => {
  it("registers desktop module only when desktop capability present", () => {
    const web = buildNavModules({ desktop: false, renderers });
    const desktop = buildNavModules({ desktop: true, renderers });
    expect(web.map((m) => m.route)).not.toContain("#/desktop");
    expect(desktop.map((m) => m.route)).toContain("#/desktop");
  });

  it("keeps stable module order with providers first", () => {
    const mods = buildNavModules({ desktop: true, renderers });
    expect(mods[0].route).toBe("#/providers");
    expect(mods.map((m) => m.route)).toEqual([
      "#/providers",
      "#/mode-routes",
      "#/security",
      "#/desktop",
      "#/general",
      "#/about",
    ]);
  });

  it("routes are unique", () => {
    const routes = buildNavModules({ desktop: true, renderers }).map((m) => m.route);
    expect(new Set(routes).size).toBe(routes.length);
  });

  it("exposes nav item testid convention", () => {
    const mods = buildNavModules({ desktop: false, renderers });
    for (const m of mods) {
      expect(m.navTestId).toBe(`nav-${m.route.replace("#/", "")}`);
    }
  });
});
