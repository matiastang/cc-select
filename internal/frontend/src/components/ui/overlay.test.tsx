import { fireEvent, render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  Progress,
  Stepper,
  Switch,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  ToastProvider,
  useToast,
} from "./overlay";

function ToastDemo() {
  const { toast } = useToast();
  return <button onClick={() => toast({ kind: "error", message: "保存失败" })}>fire</button>;
}

describe("Dialog", () => {
  it("opens from trigger and exposes title", () => {
    render(
      <Dialog>
        <DialogTrigger>打开</DialogTrigger>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>添加服务商</DialogTitle>
          </DialogHeader>
        </DialogContent>
      </Dialog>,
    );
    fireEvent.click(screen.getByRole("button", { name: "打开" }));
    expect(screen.getByRole("dialog")).toBeTruthy();
    expect(screen.getByRole("dialog", { name: "添加服务商" })).toBeTruthy();
  });
});

describe("Tabs", () => {
  it("switches content on trigger click", async () => {
    render(
      <Tabs defaultValue="a">
        <TabsList>
          <TabsTrigger value="a">A</TabsTrigger>
          <TabsTrigger value="b">B</TabsTrigger>
        </TabsList>
        <TabsContent value="a">内容A</TabsContent>
        <TabsContent value="b">内容B</TabsContent>
      </Tabs>,
    );
    expect(screen.getByText("内容A")).toBeTruthy();
    void userEvent.click(screen.getByRole("tab", { name: "B" }));
    expect(await screen.findByText("内容B")).toBeTruthy();
  });
});

describe("Switch", () => {
  it("toggles checked on click", () => {
    const onCheckedChange = vi.fn();
    render(<Switch data-testid="sw" checked={false} onCheckedChange={onCheckedChange} />);
    fireEvent.click(screen.getByTestId("sw"));
    expect(onCheckedChange).toHaveBeenCalledWith(true);
  });
});

describe("Progress", () => {
  it("exposes progressbar semantics with value", () => {
    render(<Progress value={63} data-testid="pg" aria-label="5h 用量" />);
    const pg = screen.getByRole("progressbar", { name: "5h 用量" });
    expect(pg).toHaveAttribute("aria-valuenow", "63");
  });
});

describe("Stepper", () => {
  it("renders steps with current highlighted", () => {
    render(<Stepper steps={["基础配置", "高级配置"]} current={1} data-testid="st" />);
    expect(screen.getByText("基础配置")).toBeTruthy();
    expect(screen.getByText("高级配置")).toBeTruthy();
    expect(screen.getByTestId("st-step-1")).toHaveAttribute("aria-current", "step");
    expect(screen.getByTestId("st-step-0")).not.toHaveAttribute("aria-current");
  });
});

describe("Toast", () => {
  it("renders fired toast as alert and dismisses", () => {
    render(
      <ToastProvider>
        <ToastDemo />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "fire" }));
    expect(screen.getByRole("alert")).toHaveTextContent("保存失败");
    fireEvent.click(screen.getByRole("button", { name: "close" }));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("auto dismisses after duration", () => {
    vi.useFakeTimers();
    render(
      <ToastProvider>
        <ToastDemo />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "fire" }));
    act(() => {
      vi.advanceTimersByTime(3500);
    });
    expect(screen.queryByRole("alert")).toBeNull();
    vi.useRealTimers();
  });
});
