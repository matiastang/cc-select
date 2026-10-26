import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Badge, Button, Card, EmptyState, Label, Spinner } from "./index";

describe("Button", () => {
  it("renders children and defaults to type=button", () => {
    render(<Button>保存</Button>);
    const btn = screen.getByRole("button", { name: "保存" });
    expect(btn).toHaveAttribute("type", "button");
  });

  it.each(["primary", "secondary", "danger", "ghost"] as const)(
    "applies variant class %s",
    (variant) => {
      render(
        <Button variant={variant} data-testid={`btn-${variant}`}>
          x
        </Button>,
      );
      expect(screen.getByTestId(`btn-${variant}`)).toHaveClass(`ui-button--${variant}`);
    },
  );

  it("fires onClick", () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>go</Button>);
    fireEvent.click(screen.getByRole("button", { name: "go" }));
    expect(onClick).toHaveBeenCalledOnce();
  });
});

describe("Card", () => {
  it("renders with flat modifier", () => {
    render(
      <Card flat data-testid="c">
        body
      </Card>,
    );
    expect(screen.getByTestId("c")).toHaveClass("ui-card--flat");
  });
});

describe("Badge", () => {
  it("renders text with variant class", () => {
    render(
      <Badge variant="muted" data-testid="b">
        beta
      </Badge>,
    );
    expect(screen.getByTestId("b")).toHaveTextContent("beta");
    expect(screen.getByTestId("b")).toHaveClass("ui-badge--muted");
  });
});

describe("Spinner", () => {
  it("exposes progressbar role with label", () => {
    render(<Spinner label="加载中" />);
    expect(screen.getByRole("progressbar", { name: "加载中" })).toBeTruthy();
  });
});

describe("Label", () => {
  it("associates with control via htmlFor", () => {
    render(
      <>
        <Label htmlFor="field1">名称</Label>
        <input id="field1" />
      </>,
    );
    expect(screen.getByLabelText("名称")).toBe(screen.getByRole("textbox"));
  });
});

describe("EmptyState", () => {
  it("renders title, description and fires action", () => {
    const onAction = vi.fn();
    render(
      <EmptyState
        title="还没有服务商"
        description="从预设画廊快速添加"
        actionLabel="添加服务商"
        onAction={onAction}
        data-testid="empty"
      />,
    );
    expect(screen.getByTestId("empty")).toHaveTextContent("还没有服务商");
    fireEvent.click(screen.getByRole("button", { name: "添加服务商" }));
    expect(onAction).toHaveBeenCalledOnce();
  });
});
