// 浮层/反馈类基件：Radix 无头原语（Dialog/Tabs/Switch/Tooltip）+ 自研
// （Progress/Stepper/Toast）。自研件为可访问性等价实现（role/aria 齐全），
// 规避 Radix Toast 的手势/定时器机制对老 webview 与 jsdom 的确定性风险。
import React, { createContext, useCallback, useContext, useMemo, useRef, useState } from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import * as SwitchPrimitive from "@radix-ui/react-switch";
import * as TooltipPrimitive from "@radix-ui/react-tooltip";
import { X } from "lucide-react";
import { cn } from "../../lib/cn";

/* ---------------------------------- Dialog --------------------------------- */

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

type DialogContentProps = React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content>;

export function DialogContent({ className, children, ...rest }: DialogContentProps) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="fixed inset-0 z-40 bg-black/40 data-[state=open]:animate-in data-[state=open]:fade-in-0" />
      <DialogPrimitive.Content
        className={cn(
          "fixed left-1/2 top-1/2 z-50 w-[min(92vw,560px)] max-h-[86vh] -translate-x-1/2 -translate-y-1/2",
          "overflow-y-auto rounded-lg border border-border bg-card p-5 shadow-md",
          "focus:outline-none",
          className,
        )}
        {...rest}
      >
        {children}
        <DialogPrimitive.Close
          aria-label="close"
          className="absolute right-3 top-3 rounded-sm p-1 text-muted transition-colors hover:bg-bg hover:text-text"
        >
          <X size={16} />
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

export function DialogHeader({ className, ...rest }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("mb-4 flex flex-col gap-1 pr-8", className)} {...rest} />;
}

export const DialogTitle = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Title>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...rest }, ref) => (
  <DialogPrimitive.Title
    ref={ref}
    className={cn("text-base font-semibold text-text", className)}
    {...rest}
  />
));
DialogTitle.displayName = "DialogTitle";

export const DialogDescription = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Description>,
  React.ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(({ className, ...rest }, ref) => (
  <DialogPrimitive.Description
    ref={ref}
    className={cn("text-sm text-muted", className)}
    {...rest}
  />
));
DialogDescription.displayName = "DialogDescription";

export function DialogFooter({ className, ...rest }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("mt-5 flex items-center justify-end gap-2", className)} {...rest} />;
}

/* ----------------------------------- Tabs ---------------------------------- */

export const Tabs = TabsPrimitive.Root;

export const TabsList = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.List>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.List>
>(({ className, ...rest }, ref) => (
  <TabsPrimitive.List
    ref={ref}
    className={cn(
      "inline-flex items-center gap-1 rounded-md border border-border bg-bg p-1",
      className,
    )}
    {...rest}
  />
));
TabsList.displayName = "TabsList";

export const TabsTrigger = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Trigger>
>(({ className, ...rest }, ref) => (
  <TabsPrimitive.Trigger
    ref={ref}
    className={cn(
      "rounded px-3 py-1 text-sm font-medium text-muted transition-colors",
      "hover:text-text",
      "data-[state=active]:bg-card data-[state=active]:text-text data-[state=active]:shadow-sm",
      "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50",
      className,
    )}
    {...rest}
  />
));
TabsTrigger.displayName = "TabsTrigger";

export const TabsContent = TabsPrimitive.Content;

/* ---------------------------------- Switch --------------------------------- */

type SwitchProps = React.ComponentPropsWithoutRef<typeof SwitchPrimitive.Root>;

export const Switch = React.forwardRef<React.ElementRef<typeof SwitchPrimitive.Root>, SwitchProps>(
  ({ className, ...rest }, ref) => (
    <SwitchPrimitive.Root
      ref={ref}
      className={cn(
        "peer inline-flex h-5 w-9 shrink-0 items-center rounded-full border border-transparent transition-colors",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50",
        "data-[state=checked]:bg-accent data-[state=unchecked]:bg-border",
        "disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...rest}
    >
      <SwitchPrimitive.Thumb className="block h-4 w-4 translate-x-0.5 rounded-full bg-card shadow transition-transform data-[state=checked]:translate-x-[18px]" />
    </SwitchPrimitive.Root>
  ),
);
Switch.displayName = "Switch";

/* --------------------------------- Tooltip --------------------------------- */

export function TooltipProvider({ children }: { children: React.ReactNode }) {
  return <TooltipPrimitive.Provider delayDuration={200}>{children}</TooltipPrimitive.Provider>;
}

export function Tooltip({
  label,
  children,
}: {
  label: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <TooltipPrimitive.Root>
      <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content
          sideOffset={6}
          className="z-50 rounded-md border border-border bg-card px-2 py-1 text-xs text-text shadow-md"
        >
          {label}
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}

/* --------------------------------- Progress -------------------------------- */

type ProgressProps = {
  value: number; // 0–100
  "aria-label"?: string;
  className?: string;
  "data-testid"?: string;
};

export function Progress({ value, className, ...rest }: ProgressProps) {
  const clamped = Math.max(0, Math.min(100, Math.round(value)));
  return (
    <div
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={clamped}
      className={cn("h-2 w-full overflow-hidden rounded-full bg-border", className)}
      {...rest}
    >
      <div
        className={cn(
          "h-full rounded-full transition-[width]",
          clamped >= 90 ? "bg-danger" : "bg-accent",
        )}
        style={{ width: `${clamped}%` }}
      />
    </div>
  );
}

/* --------------------------------- Stepper --------------------------------- */

type StepperProps = {
  steps: React.ReactNode[];
  current: number;
  className?: string;
  "data-testid"?: string;
};

export function Stepper({ steps, current, className, ...rest }: StepperProps) {
  return (
    <ol className={cn("flex items-center gap-2 text-sm", className)} {...rest}>
      {steps.map((label, i) => {
        const state = i < current ? "done" : i === current ? "current" : "todo";
        return (
          <li
            key={i}
            data-testid={`${rest["data-testid"]}-step-${i}`}
            aria-current={state === "current" ? "step" : undefined}
            className={cn(
              "flex items-center gap-1.5 rounded-md px-2 py-1",
              state === "current" && "bg-notice font-medium text-accent",
              state === "done" && "text-muted",
              state === "todo" && "text-muted/70",
            )}
          >
            <span
              className={cn(
                "flex h-5 w-5 items-center justify-center rounded-full border text-xs",
                state === "current" && "border-accent text-accent",
                state === "done" && "border-border",
                state === "todo" && "border-border",
              )}
            >
              {i + 1}
            </span>
            <span>{label}</span>
          </li>
        );
      })}
    </ol>
  );
}

/* ---------------------------------- Toast ---------------------------------- */

export type ToastKind = "info" | "success" | "error";

export interface ToastItem {
  id: number;
  kind: ToastKind;
  message: string;
}

interface ToastContextValue {
  toast: (t: { kind?: ToastKind; message: string; durationMs?: number }) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within ToastProvider");
  return ctx;
}

const TOAST_STYLES: Record<ToastKind, string> = {
  info: "border-border bg-card text-text",
  success: "border-emerald-500/40 bg-card text-text",
  error: "border-danger/40 bg-notice-danger text-danger",
};

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const nextId = useRef(1);

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const toast = useCallback<ToastContextValue["toast"]>(
    ({ kind = "info", message, durationMs = 3000 }) => {
      const id = nextId.current++;
      setToasts((prev) => [...prev.slice(-3), { id, kind, message }]);
      window.setTimeout(() => dismiss(id), durationMs);
    },
    [dismiss],
  );

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="fixed bottom-4 right-4 z-[60] flex w-[min(92vw,360px)] flex-col gap-2">
        {toasts.map((t) => (
          <div
            key={t.id}
            role={t.kind === "error" ? "alert" : "status"}
            className={cn(
              "flex items-start gap-2 rounded-md border px-3 py-2 text-sm shadow-md",
              TOAST_STYLES[t.kind],
            )}
          >
            <span className="flex-1 break-words">{t.message}</span>
            <button
              type="button"
              aria-label="close"
              onClick={() => dismiss(t.id)}
              className="rounded p-0.5 text-muted hover:text-text"
            >
              <X size={14} />
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
