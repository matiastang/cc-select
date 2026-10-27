import React from "react";
import { cn } from "../../lib/cn";

type BadgeVariant = "default" | "muted" | "success" | "danger";

const VARIANT_CLASSES: Record<BadgeVariant, string> = {
  default: "border-accent/30 bg-notice text-accent",
  muted: "border-border bg-bg text-muted",
  success: "border-emerald-500/30 bg-emerald-500/10 text-emerald-600",
  danger: "border-danger/30 bg-notice-danger text-danger",
};

type BadgeProps = React.HTMLAttributes<HTMLSpanElement> & {
  variant?: BadgeVariant;
};

export function Badge({ variant = "default", className, ...rest }: BadgeProps) {
  return (
    <span
      className={cn(
        "ui-badge",
        `ui-badge--${variant}`,
        "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium",
        VARIANT_CLASSES[variant],
        className,
      )}
      {...rest}
    />
  );
}
