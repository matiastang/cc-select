import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Boxes, Info } from "lucide-react";
import { AppShell } from "./AppShell";

function makeModules() {
  return [
    {
      route: "#/providers",
      titleKey: "nav.providers",
      icon: Boxes,
      render: () => <div data-testid="mod-providers">providers content</div>,
      navTestId: "nav-providers",
    },
    {
      route: "#/about",
      titleKey: "nav.about",
      icon: Info,
      render: () => <div data-testid="mod-about">about content</div>,
      navTestId: "nav-about",
    },
  ];
}

describe("AppShell", () => {
  it("renders app title heading and all nav items", () => {
    render(<AppShell modules={makeModules()} route="#/providers" onNavigate={vi.fn()} />);
    expect(screen.getByRole("heading", { level: 1 })).toBeTruthy();
    expect(screen.getByTestId("nav-providers")).toBeTruthy();
    expect(screen.getByTestId("nav-about")).toBeTruthy();
  });

  it("marks active nav item with aria-current and renders its module", () => {
    render(<AppShell modules={makeModules()} route="#/providers" onNavigate={vi.fn()} />);
    expect(screen.getByTestId("nav-providers")).toHaveAttribute("aria-current", "page");
    expect(screen.getByTestId("nav-about")).not.toHaveAttribute("aria-current");
    expect(screen.getByTestId("mod-providers")).toBeTruthy();
  });

  it("navigates on nav item click", () => {
    const onNavigate = vi.fn();
    render(<AppShell modules={makeModules()} route="#/providers" onNavigate={onNavigate} />);
    fireEvent.click(screen.getByTestId("nav-about"));
    expect(onNavigate).toHaveBeenCalledWith("#/about");
  });

  it("exposes header controls slot", () => {
    render(
      <AppShell modules={makeModules()} route="#/providers" onNavigate={vi.fn()}>
        <button>ctrl</button>
      </AppShell>,
    );
    expect(
      screen.getByTestId("header-controls").contains(screen.getByRole("button", { name: "ctrl" })),
    ).toBe(true);
  });
});
